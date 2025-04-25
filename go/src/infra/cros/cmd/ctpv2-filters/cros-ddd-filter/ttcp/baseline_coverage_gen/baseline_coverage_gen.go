// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// This cli computes an null coverage file for a set of source files given in
// parameters.
//
// The reason this CLI was written is because the golang coverage used within
// the context of bazel only report coverage for file for the tests that the
// coverage command triggers. This hides source files that do not have tests
// that can reach them and computes an errornous code coverage ratio.
package main

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"os"
	"path"
	"reflect"
	"runtime/debug"
	"strings"

	args "github.com/alexflint/go-arg"
)

type CliArgs struct {
	IncludeMain         bool   `arg:"-m,--include_main"  help:"add zero coverage for the main function"`
	Pwd                 string `arg:"positional,required" help:"in: absolute path of bazel's workspace directory"`
	GoSourceFileSetFile string `arg:"positional,required" help:"in: dataset of source files"`
	BaseLineFile        string `arg:"positional,required" help:"out: path of the null basecoverage file"`
}

func (CliArgs) Description() string {
	return `This baseline_coverage_gen is meant to be run in the following way:
  - Place your prompt at the Bazel workspace directory of your project
  - Capture all the go source files present in the workspace with the command:
    bazel query 'filter(".go$",labels(srcs, //...))' > tmp_go_source_files_set.
	This creates the dataset of source files: the data set is a text file where
	for each source file, there is one line in the dataset containing its path
	relative to the workspace directory.
  - Generate an null baseline tracefile with baseline_coverage_gen/
    baseline_coverage_gen $(pwd) $tmp_go_source_files_set tmp_go_baseline_coverage
  - Generate the code coverage for the full bazel project
    bazelisk coverage  --test_output=all   --combined_report=lcov //...
  - Combine the baseline and the test coverage
    lcov -a "$(bazel info output_path)/_coverage/_coverage_report.dat" -a tmp_go_baseline_coverage -o tmp_go_fullcoverage > $tmp_lcov_merge_stream
    This will output now the correct coverage ration for the project
  - [optionally] to generate a graphical code coverage report
    genhtml -s -o tmp_report_dir tmp_go_fullcoverage`
}

const FilePermission_User_RW_Group_R_Other_R = 0644

func main() {
	mainInt(os.Args[1:])
}

// This is part of the main function but isolated in its own function
// in order to test it.
func mainInt(osargs []string) {
	var cliArgs CliArgs
	parser, err := args.NewParser(args.Config{Program: "", IgnoreEnv: true}, &cliArgs)
	if err != nil {
		fmt.Println("Error in building cli command parser:", err)
		os.Exit(1)
	}
	err = parser.Parse(osargs)
	if err != nil {
		fmt.Println("Error in parsing cli arguments:", err)
		os.Exit(1)
	}

	goFiles := extractFileSet(cliArgs.GoSourceFileSetFile)
	var baseLineFileContent strings.Builder
	for _, goFile := range goFiles {
		// we skip computing the coverage of the test files.
		if strings.HasSuffix(goFile, "_test.go") {
			continue
		}
		// We collect the null coverage for each file and aggregate it.
		section := extractBaseLine(cliArgs.Pwd, goFile, cliArgs.IncludeMain)
		for _, line := range section {
			fmt.Fprintf(&baseLineFileContent, "%s\n", line)
		}
	}

	err = os.WriteFile(cliArgs.BaseLineFile, []byte(baseLineFileContent.String()),
		FilePermission_User_RW_Group_R_Other_R)
	if err != nil {
		log.Fatal("error writing the null coverage tracefile:", err)
	}
}

// extractFileSet reads a file of one bazel target per line and transforms into
// relative file paths. extractFileSet returns the list the relative file paths.
func extractFileSet(path string) []string {
	readFile, err := os.Open(path)

	if err != nil {
		log.Fatal("error in reading source file set data file:", err)
	}
	fileScanner := bufio.NewScanner(readFile)
	fileScanner.Split(bufio.ScanLines)
	files := []string{}
	for fileScanner.Scan() {
		// Bazel outputs the targe for each file as
		// "//"+subfolde_path+":"+relative_path
		// Here we strip the prefix "//" and replace the ":" to make it a valid path
		// relative to the workspace directory.
		bazelTarget := fileScanner.Text()
		path := strings.ReplaceAll(bazelTarget[2:], ":", "/")
		log.Println("bazelTarget: ", bazelTarget)
		log.Println("       path: ", path)
		files = append(files, path)
	}
	readFile.Close()
	return files
}

const AbsolutePathToTheSourceFile = "SF"
const NumberOfFunctionsFound = "FNF"
const NumberOfFunctionsHit = "FNH"
const NumberOfInstrumentedLines = "LF"
const NumberOfLinesWithANonZeroExecutionCount = "LH"
const InstrumentedLineHitCount = "DA"

// extractFileSet generates an lcov trace file section as specified in the lcov
// man page (https://ltp.sourceforge.net/coverage/lcov/geninfo.1.php). As the
// golang coverage tool only generates line based coverage also only generates
// a coverage only for the lines.
func extractBaseLine(pwd string, goFile string, includeMainFunction bool) []string {
	section := []string{
		AbsolutePathToTheSourceFile + ":" + goFile,
		NumberOfFunctionsFound + ":0",
		NumberOfFunctionsHit + ":0",
	}
	sourceFilePath := path.Join(pwd, goFile)
	// We parse the golang source file into an AST
	fset := token.NewFileSet()
	syntaxTree, error := parser.ParseFile(fset, sourceFilePath, nil, 0)
	if error != nil {
		log.Fatal("Error in parsing file ", goFile, " error:", error)
	}
	// We iterate over each declaration of the file, compute the null coverage
	// and accumulate it.
	nbFunctions := 0
	for _, untypedDcl := range syntaxTree.Decls {
		switch dcl := untypedDcl.(type) {
		case *ast.FuncDecl:
			for codeLine := range codeLinesOfFunction(fset, dcl, includeMainFunction) {
				section = append(section, fmt.Sprintf("%s:%d,0", InstrumentedLineHitCount, codeLine))
			}
			nbFunctions++
		case *ast.GenDecl:
			// GenDecl represent import, constant, type or variable declarations
			// This is not executable instructions and excluded of the coverage.
			continue
		default:
			// By default we crash the program for any element that program
			// currently is not explicitly designed to process.
			log.Fatal("Internal Error: Ignored DCL type:", reflect.TypeOf(dcl))
		}
	}
	section = append(section, NumberOfLinesWithANonZeroExecutionCount+":0")
	section = append(section, NumberOfInstrumentedLines+":0")
	section = append(section, "end_of_record")
	return section
}

// codeLinesOfFunction computes the coverage of a function declaration
// Arguments:
//
//	fst :   Is the FileSet used to parse the AST, it is used to map the AST back
//	        to the lines of a file.
//	funcDcl:Is the function to compute the null coverage for.
//
// Returns: a hashmap of the lines that contain code.
func codeLinesOfFunction(fst *token.FileSet, funcDcl *ast.FuncDecl, includeMainFunction bool) map[int]struct{} {
	codeLines := map[int]struct{}{}
	if !includeMainFunction && funcDcl.Name.Name == "main" {
		return codeLines
	}
	codeLinesOfBlock(fst, funcDcl.Body, codeLines)
	return codeLines
}

// codeLinesOfBlock computes the coverage of a block of statments
// Arguments:
//
//	fst: Is the FileSet used to parse the AST, it is used to  map the AST back
//	     to the lines of a file.
//	block: Is the block to compute the null coverage for.
//	codeLine: is the hashset of line to report the line coverage to.
func codeLinesOfBlock(fst *token.FileSet, block *ast.BlockStmt, codeLine map[int]struct{}) {
	for _, untypedStmt := range block.List {
		codeLinesOfStmt(fst, untypedStmt, codeLine)
	}
}

// codeLinesOfStmt computes the coverage of a block of a statment
// Arguments:
//
//	fst: Is the FileSet used to parse the AST, it is used to  map the AST back
//	     to the lines of a file.
//	untypedStmt: Is the statment to compute the null coverage for.
//	codeLine: is the hashset of line to report the line coverage to.
func codeLinesOfStmt(fst *token.FileSet, untypedStmt ast.Stmt, codeLines map[int]struct{}) {
	switch stmt := untypedStmt.(type) {
	case *ast.BlockStmt:
		codeLinesOfBlock(fst, stmt, codeLines)
	case *ast.ExprStmt:
		codeLinesOfExpression(fst, stmt.X, codeLines)
	case *ast.AssignStmt:
		for _, expr := range stmt.Lhs {
			codeLinesOfExpression(fst, expr, codeLines)
		}
		codeLines[fst.Position(stmt.TokPos).Line] = struct{}{}
		for _, expr := range stmt.Rhs {
			codeLinesOfExpression(fst, expr, codeLines)
		}
	case *ast.DeferStmt:
		codeLines[fst.Position(stmt.Defer).Line] = struct{}{}
		codeLinesOfCallExpression(fst, stmt.Call, codeLines)
	case *ast.ReturnStmt:
		codeLines[fst.Position(stmt.Return).Line] = struct{}{}
		for _, expr := range stmt.Results {
			codeLinesOfExpression(fst, expr, codeLines)
		}
	case *ast.RangeStmt:
		codeLines[fst.Position(stmt.For).Line] = struct{}{}
		codeLinesOfExpression(fst, stmt.Key, codeLines)
		if stmt.Value != nil {
			codeLinesOfExpression(fst, stmt.Value, codeLines)
		}
		codeLines[fst.Position(stmt.TokPos).Line] = struct{}{}
		codeLinesOfExpression(fst, stmt.X, codeLines)
		codeLinesOfBlock(fst, stmt.Body, codeLines)
	case *ast.IfStmt:
		codeLines[fst.Position(stmt.If).Line] = struct{}{}
		if stmt.Init != nil {
			codeLinesOfStmt(fst, stmt.Init, codeLines)
		}
		codeLinesOfExpression(fst, stmt.Cond, codeLines)
		codeLinesOfBlock(fst, stmt.Body, codeLines)
		if stmt.Else != nil {
			codeLinesOfStmt(fst, stmt.Else, codeLines)
		}
	case *ast.SwitchStmt:
		codeLines[fst.Position(stmt.Switch).Line] = struct{}{}
		if stmt.Init != nil {
			codeLinesOfStmt(fst, stmt.Init, codeLines)
		}
		if stmt.Tag != nil {
			codeLinesOfExpression(fst, stmt.Tag, codeLines)
		}
		codeLinesOfBlock(fst, stmt.Body, codeLines)
	case *ast.TypeSwitchStmt:
		codeLines[fst.Position(stmt.Switch).Line] = struct{}{}
		if stmt.Init != nil {
			codeLinesOfStmt(fst, stmt.Init, codeLines)
		}
		codeLinesOfStmt(fst, stmt.Assign, codeLines)
		codeLinesOfBlock(fst, stmt.Body, codeLines)
	case *ast.DeclStmt:
		return
	case *ast.CaseClause:
		codeLines[fst.Position(stmt.Case).Line] = struct{}{}
		for _, expr := range stmt.List {
			codeLinesOfExpression(fst, expr, codeLines)
		}
		codeLines[fst.Position(stmt.Colon).Line] = struct{}{}
		for _, substmt := range stmt.Body {
			codeLinesOfStmt(fst, substmt, codeLines)
		}
	case *ast.ForStmt:
		if stmt.Init != nil {
			codeLinesOfStmt(fst, stmt.Init, codeLines)
		}
		if stmt.Cond != nil {
			codeLinesOfExpression(fst, stmt.Cond, codeLines)
		}
		if stmt.Post != nil {
			codeLinesOfStmt(fst, stmt.Post, codeLines)
		}
		codeLinesOfBlock(fst, stmt.Body, codeLines)
	case *ast.IncDecStmt:
		codeLinesOfExpression(fst, stmt.X, codeLines)
	case *ast.BranchStmt:
		codeLines[fst.Position(stmt.TokPos).Line] = struct{}{}
	case *ast.LabeledStmt:
		codeLinesOfStmt(fst, stmt.Stmt, codeLines)
	case *ast.GoStmt:
		codeLines[fst.Position(stmt.Go).Line] = struct{}{}
		codeLinesOfCallExpression(fst, stmt.Call, codeLines)
	case *ast.SendStmt:
		codeLinesOfExpression(fst, stmt.Chan, codeLines)
		codeLines[fst.Position(stmt.Arrow).Line] = struct{}{}
		codeLinesOfExpression(fst, stmt.Value, codeLines)

	case nil:
		// This case should not happen, if it happens it is defect that needs
		// to be corrected.
		debug.PrintStack()
		log.Fatal("Internal Error:null statment error.")
	default:
		// By default we crash the program for any element that program
		// currently is not explicitly designed to process.
		debug.PrintStack()
		log.Fatal("Internal error,  statment of type not recognized:", reflect.TypeOf(stmt))
	}
}

// codeLinesOfCallExpression computes the coverage of a block of a function call
// Arguments:
//
//	fst: Is the FileSet used to parse the AST, it is used to  map the AST back
//	     to the lines of a file.
//	call: Is the function call to compute the null coverage for.
//	codeLine: is the hashset of line to report the line coverage to.
func codeLinesOfCallExpression(fst *token.FileSet, call *ast.CallExpr, codeLines map[int]struct{}) {
	codeLinesOfExpression(fst, call.Fun, codeLines)
	codeLines[fst.Position(call.Lparen).Line] = struct{}{}
	for _, expr := range call.Args {
		codeLinesOfExpression(fst, expr, codeLines)
	}
	if call.Ellipsis != token.NoPos {
		codeLines[fst.Position(call.Ellipsis).Line] = struct{}{}
	}
	codeLines[fst.Position(call.Rparen).Line] = struct{}{}
}

// codeLinesOfExpression computes the coverage of a block of an expression
// Returns: a hashmap of the lines that contain code.
func codeLinesOfExpression(fst *token.FileSet, untypedExpr ast.Expr, codeLines map[int]struct{}) {
	switch expr := untypedExpr.(type) {
	case *ast.CallExpr:
		codeLinesOfCallExpression(fst, expr, codeLines)
	case *ast.Ident:
		codeLines[fst.Position(expr.NamePos).Line] = struct{}{}
	case *ast.SelectorExpr:
		codeLinesOfExpression(fst, expr.X, codeLines)
		codeLines[fst.Position(expr.Sel.NamePos).Line] = struct{}{}
	case *ast.BinaryExpr:
		codeLinesOfExpression(fst, expr.X, codeLines)
		codeLines[fst.Position(expr.OpPos).Line] = struct{}{}
		codeLinesOfExpression(fst, expr.Y, codeLines)
	case *ast.BasicLit:
		codeLines[fst.Position(expr.ValuePos).Line] = struct{}{}
	case *ast.IndexExpr:
		codeLinesOfExpression(fst, expr.X, codeLines)
		codeLines[fst.Position(expr.Lbrack).Line] = struct{}{}
		codeLinesOfExpression(fst, expr.Index, codeLines)
		codeLines[fst.Position(expr.Rbrack).Line] = struct{}{}
	case *ast.UnaryExpr:
		codeLines[fst.Position(expr.OpPos).Line] = struct{}{}
		codeLinesOfExpression(fst, expr.X, codeLines)
	case *ast.CompositeLit:
		if expr.Type != nil {
			codeLinesOfExpression(fst, expr.Type, codeLines)
		}
		codeLines[fst.Position(expr.Lbrace).Line] = struct{}{}
		for _, subexpr := range expr.Elts {
			codeLinesOfExpression(fst, subexpr, codeLines)
		}
		codeLines[fst.Position(expr.Rbrace).Line] = struct{}{}
	case *ast.SliceExpr:
		codeLinesOfExpression(fst, expr.X, codeLines)
		codeLines[fst.Position(expr.Lbrack).Line] = struct{}{}
		if expr.Low != nil {
			codeLinesOfExpression(fst, expr.Low, codeLines)
		}
		if expr.High != nil {
			codeLinesOfExpression(fst, expr.High, codeLines)
		}
		if expr.Max != nil {
			codeLinesOfExpression(fst, expr.Max, codeLines)
		}
		codeLines[fst.Position(expr.Rbrack).Line] = struct{}{}
	case *ast.KeyValueExpr:
		codeLinesOfExpression(fst, expr.Key, codeLines)
		codeLines[fst.Position(expr.Colon).Line] = struct{}{}
		codeLinesOfExpression(fst, expr.Value, codeLines)
	case *ast.TypeAssertExpr:
		codeLinesOfExpression(fst, expr.X, codeLines)
		codeLines[fst.Position(expr.Lparen).Line] = struct{}{}
		if expr.Type != nil {
			codeLinesOfExpression(fst, expr.Type, codeLines)
		}
		codeLines[fst.Position(expr.Rparen).Line] = struct{}{}
	case *ast.FuncLit:
		codeLinesOfBlock(fst, expr.Body, codeLines)
	case *ast.MapType:
		return
	case *ast.ArrayType:
		return
	case *ast.StructType:
		return
	case *ast.ParenExpr:
		codeLinesOfExpression(fst, expr.X, codeLines)
	case *ast.StarExpr:
		codeLines[fst.Position(expr.Star).Line] = struct{}{}
		codeLinesOfExpression(fst, expr.X, codeLines)
	case *ast.ChanType:
		codeLinesOfExpression(fst, expr.Value, codeLines)
	case nil:
		// This case should not happen, if it happens it is defect that needs
		// to be corrected.
		debug.PrintStack()
		log.Fatal("Internal error: null expression error.")
	default:
		// By default we crash the program for any element that program
		// currently is not explicitly designed to process.
		debug.PrintStack()
		log.Fatal("Internal error, expression of type not recognized:", reflect.TypeOf(expr))
	}
}
