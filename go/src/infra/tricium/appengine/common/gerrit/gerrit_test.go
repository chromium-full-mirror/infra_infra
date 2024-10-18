// Copyright 2017 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package gerrit

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	tricium "infra/tricium/api/v1"
	"infra/tricium/appengine/common/triciumtest"
)

func TestCreateRobotComment(t *testing.T) {
	ftt.Run("Test Environment", t, func(t *ftt.Test) {

		ctx := triciumtest.Context()
		runID := int64(1234567)
		uuid := "7ae6f43d-22e9-4350-ace4-1fee9014509a"

		t.Run("Basic comment fields include UUID and URL", func(t *ftt.Test) {
			roco := createRobotComment(ctx, runID, &tricium.Data_Comment{
				Id:       uuid,
				Path:     "README.md",
				Message:  "Message",
				Category: "Hello",
			})
			assert.Loosely(t, roco, should.Resemble(&robotCommentInput{
				Message:    "Message",
				RobotID:    "Hello",
				RobotRunID: "1234567",
				URL:        "https://app.example.com/run/1234567",
				Path:       "README.md",
				Properties: map[string]string{
					"tricium_comment_uuid": "7ae6f43d-22e9-4350-ace4-1fee9014509a",
				},
			}))
		})

		t.Run("File comment has no position info", func(t *ftt.Test) {
			roco := createRobotComment(ctx, runID, &tricium.Data_Comment{
				Id:       uuid,
				Path:     "README.md",
				Message:  "Message",
				Category: "Hello",
			})
			assert.Loosely(t, roco.Line, should.BeZero)
			assert.Loosely(t, roco.Range, should.BeNil)
		})

		t.Run("Line comment has no range info", func(t *ftt.Test) {
			line := 10
			roco := createRobotComment(ctx, runID, &tricium.Data_Comment{
				Id:        uuid,
				Path:      "README.md",
				Message:   "Message",
				Category:  "Hello",
				StartLine: int32(line),
			})
			assert.That(t, roco.Line, should.Equal(line))
			assert.Loosely(t, roco.Range, should.BeNil)
		})

		t.Run("Range comment has range", func(t *ftt.Test) {
			startLine := 10
			endLine := 20
			startChar := 2
			endChar := 18
			roco := createRobotComment(ctx, runID, &tricium.Data_Comment{
				Id:        uuid,
				Path:      "README.md",
				Message:   "Message",
				Category:  "Hello",
				StartLine: int32(startLine),
				EndLine:   int32(endLine),
				StartChar: int32(startChar),
				EndChar:   int32(endChar),
			})
			assert.Loosely(t, roco.Line, should.Equal(endLine))
			assert.Loosely(t, roco.Range, should.Resemble(&commentRange{
				StartLine:      startLine,
				EndLine:        endLine,
				StartCharacter: startChar,
				EndCharacter:   endChar,
			}))
		})

		t.Run("Comment gets marshaled correctly", func(t *ftt.Test) {
			startLine := 10
			endLine := 20
			startChar := 2
			endChar := 18
			roco := createRobotComment(ctx, runID, &tricium.Data_Comment{
				Id:        uuid,
				Path:      "README.md",
				Message:   "Message",
				Category:  "Hello",
				StartLine: int32(startLine),
				EndLine:   int32(endLine),
				StartChar: int32(startChar),
				EndChar:   int32(endChar),
				Suggestions: []*tricium.Data_Suggestion{
					{
						Replacements: []*tricium.Data_Replacement{
							{
								Replacement: "",
								Path:        "README.md",
								StartLine:   int32(startLine),
								EndLine:     int32(endLine),
								StartChar:   int32(startChar),
								EndChar:     int32(endChar),
							},
						},
						Description: "Suggestion 1",
					},
				},
			})

			marshaledRoco, _ := json.MarshalIndent(roco, "", "  ")
			assert.Loosely(t, string(marshaledRoco), should.Equal(`{
  "robot_id": "Hello",
  "robot_run_id": "1234567",
  "url": "https://app.example.com/run/1234567",
  "properties": {
    "tricium_comment_uuid": "7ae6f43d-22e9-4350-ace4-1fee9014509a"
  },
  "fix_suggestions": [
    {
      "description": "Suggestion 1",
      "replacements": [
        {
          "path": "README.md",
          "replacement": "",
          "range": {
            "start_line": 10,
            "start_character": 2,
            "end_line": 20,
            "end_character": 18
          }
        }
      ]
    }
  ],
  "path": "README.md",
  "line": 20,
  "range": {
    "start_line": 10,
    "start_character": 2,
    "end_line": 20,
    "end_character": 18
  },
  "message": "Message"
}`))

		})
	})
}

func TestGetChangedLinesFromPatch(t *testing.T) {
	ftt.Run("Extract changed lines with added and modified files", t, func(t *ftt.Test) {
		patch := `commit 29943c31812f582bd174740d9f9414a99632c687 (HEAD -> master)
Author: Foo Bar <foobar@google.com>
Date:   Tue Jun 26 17:58:25 2018 -0700

    new commit

diff --git a/test.cpp b/test.cpp
new file mode 100644
index 0000000..382e810
--- /dev/null
+++ b/test.cpp
@@ -0,0 +1,6 @@
+#include <iostream>
+
+int main(int argc, char **arg) {
+  std::cout << "Hello, World!";
+  return 0;
+}
diff --git a/test2.cpp b/test2.cpp
new file mode 100644
index 0000000..ab8dadd
--- a/test2.cpp
+++ b/test2.cpp
@@ -1,7 +1,7 @@
 #include <iostream>

-int main() {
+int main(int argc, char **arg) {
+
   std::cout << "Hello, World!";
-  return 0;
 }
`

		base64Patch := base64.StdEncoding.EncodeToString([]byte(patch))
		expectedLines := ChangedLinesInfo{}
		expectedLines["test.cpp"] = []int{1, 2, 3, 4, 5, 6}
		expectedLines["test2.cpp"] = []int{2, 3}
		actualLines, err := getChangedLinesFromPatch(base64Patch)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, actualLines, should.Resemble(expectedLines))
	})

	ftt.Run("Extract changed lines with a deleted file", t, func(t *ftt.Test) {
		patch := `commit 29943c31812f582bd174740d9f9414a99632c687 (HEAD -> master)
Author: Foo Bar <foobar@google.com>
Date:   Tue Jun 26 17:58:25 2018 -0700

		delete commit

diff --git a/test.cpp b/test.cpp
deleted file mode 100644
index 382e810..0000000
--- a/test.cpp
+++ /dev/null
@@ -1,6 +0,0 @@
-#include <iostream>
-
-int main(int argc, char **arg) {
-  std::cout << "Hello, World!";
-  return 0;
-}
`

		base64Patch := base64.StdEncoding.EncodeToString([]byte(patch))
		expectedLines := ChangedLinesInfo{}
		actualLines, err := getChangedLinesFromPatch(base64Patch)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, actualLines, should.Resemble(expectedLines))
	})
}

func TestCommentIsInChangedLines(t *testing.T) {
	ftt.Run("Test Environment", t, func(t *ftt.Test) {

		ctx := triciumtest.Context()

		t.Run("Anything in CL description is included", func(t *ftt.Test) {
			data := tricium.Data_Comment{
				Path: "",
			}
			lines := ChangedLinesInfo{"": {2, 5, 10}}
			assert.Loosely(t, CommentIsInChangedLines(ctx, &data, lines, 0), should.BeTrue)
		})

		t.Run("Single line comment in changed lines", func(t *ftt.Test) {
			data := tricium.Data_Comment{
				Path:      "dir/file.txt",
				StartLine: 5,
				EndLine:   5,
				StartChar: 0,
				EndChar:   10,
			}
			lines := ChangedLinesInfo{"dir/file.txt": {2, 5, 10}}
			assert.Loosely(t, CommentIsInChangedLines(ctx, &data, lines, 0), should.BeTrue)
		})

		t.Run("Single line comment outside of changed lines", func(t *ftt.Test) {
			data := tricium.Data_Comment{
				Path:      "dir/file.txt",
				StartLine: 4,
				EndLine:   4,
				StartChar: 0,
				EndChar:   10,
			}
			lines := ChangedLinesInfo{"dir/file.txt": {2, 5, 10}}
			assert.Loosely(t, CommentIsInChangedLines(ctx, &data, lines, 0), should.BeFalse)
		})

		t.Run("Single line comment outside of changed files", func(t *ftt.Test) {
			data := tricium.Data_Comment{
				Path:      "DELETED.txt",
				StartLine: 5,
				EndLine:   5,
				StartChar: 5,
				EndChar:   10,
			}
			lines := ChangedLinesInfo{"dir/file.txt": {2, 5, 10}}
			assert.Loosely(t, CommentIsInChangedLines(ctx, &data, lines, 0), should.BeFalse)
		})

		t.Run("Comment with line range that overlaps changed line", func(t *ftt.Test) {
			data := tricium.Data_Comment{
				Path:      "dir/file.txt",
				StartLine: 3,
				EndLine:   8,
			}
			lines := ChangedLinesInfo{"dir/file.txt": {2, 5, 10}}
			assert.Loosely(t, CommentIsInChangedLines(ctx, &data, lines, 0), should.BeTrue)
		})

		t.Run("Comment with end char == 0, implying end line is not included", func(t *ftt.Test) {
			data := tricium.Data_Comment{
				Path:      "dir/file.txt",
				StartLine: 6,
				EndLine:   10,
			}
			lines := ChangedLinesInfo{"dir/file.txt": {2, 5, 10}}
			assert.Loosely(t, CommentIsInChangedLines(ctx, &data, lines, 0), should.BeFalse)
		})

		t.Run("File-level comments are included", func(t *ftt.Test) {
			data := tricium.Data_Comment{
				Path: "dir/file.txt",
			}
			lines := ChangedLinesInfo{"dir/file.txt": {2, 5, 10}}
			assert.Loosely(t, CommentIsInChangedLines(ctx, &data, lines, 0), should.BeTrue)
		})

		t.Run("Line comments on changed lines are included", func(t *ftt.Test) {
			data := tricium.Data_Comment{
				Path:      "dir/file.txt",
				StartLine: 2,
			}
			lines := ChangedLinesInfo{"dir/file.txt": {2, 5, 10}}
			assert.Loosely(t, CommentIsInChangedLines(ctx, &data, lines, 0), should.BeTrue)
		})

		// Fuzzy matching.

		t.Run("Line comments not close before changed lines are excluded", func(t *ftt.Test) {
			data := tricium.Data_Comment{
				Path:      "dir/file.txt",
				StartLine: 1,
			}
			lines := ChangedLinesInfo{"dir/file.txt": {3, 5, 10}}
			assert.Loosely(t, CommentIsInChangedLines(ctx, &data, lines, 1), should.BeFalse)
		})

		t.Run("Line comments not close after changed lines are excluded", func(t *ftt.Test) {
			data := tricium.Data_Comment{
				Path:      "dir/file.txt",
				StartLine: 7,
			}
			lines := ChangedLinesInfo{"dir/file.txt": {3, 5, 10}}
			assert.Loosely(t, CommentIsInChangedLines(ctx, &data, lines, 1), should.BeFalse)
		})

		t.Run("Line comments close before changed lines are included", func(t *ftt.Test) {
			data := tricium.Data_Comment{
				Path:      "dir/file.txt",
				StartLine: 2,
			}
			lines := ChangedLinesInfo{"dir/file.txt": {3, 5, 10}}
			assert.Loosely(t, CommentIsInChangedLines(ctx, &data, lines, 1), should.BeTrue)
		})

		t.Run("Line comments close after changed lines are included", func(t *ftt.Test) {
			data := tricium.Data_Comment{
				Path:      "dir/file.txt",
				StartLine: 6,
			}
			lines := ChangedLinesInfo{"dir/file.txt": {2, 5, 10}}
			assert.Loosely(t, CommentIsInChangedLines(ctx, &data, lines, 1), should.BeTrue)
		})
	})
}

func TestIsInChangedLines(t *testing.T) {
	ftt.Run("Overlapping cases", t, func(t *ftt.Test) {
		assert.Loosely(t, isInChangedLines(1, 3, []int{2, 3, 4}, 0), should.BeTrue)
		assert.Loosely(t, isInChangedLines(4, 5, []int{2, 3, 4}, 0), should.BeTrue)
		// The end line is inclusive.
		assert.Loosely(t, isInChangedLines(1, 2, []int{2, 3, 4}, 0), should.BeTrue)
		assert.Loosely(t, isInChangedLines(3, 3, []int{2, 3, 4}, 0), should.BeTrue)
	})

	ftt.Run("Non-overlapping cases", t, func(t *ftt.Test) {
		assert.Loosely(t, isInChangedLines(5, 6, []int{2, 3, 4}, 0), should.BeFalse)
		assert.Loosely(t, isInChangedLines(1, 1, []int{2, 3, 4}, 0), should.BeFalse)
	})

	ftt.Run("Invalid range cases", t, func(t *ftt.Test) {
		assert.Loosely(t, isInChangedLines(2, 0, []int{2, 3, 4}, 0), should.BeFalse)
	})
}

func TestAdjustCommitMessage(t *testing.T) {
	ftt.Run("adjustCommitMessageComment changes positions in comment and replacements", t, func(t *ftt.Test) {
		comment := &tricium.Data_Comment{
			Category:  "Foo",
			Message:   "Bar",
			StartLine: 5,
			EndLine:   5,
			StartChar: 2,
			EndChar:   4,
			Suggestions: []*tricium.Data_Suggestion{
				{
					Replacements: []*tricium.Data_Replacement{
						{
							StartLine: 5,
							EndLine:   5,
							StartChar: 2,
							EndChar:   4,
						},
					},
				},
			},
		}
		adjustCommitMessageComment(comment)
		assert.Loosely(t, comment, should.Resemble(&tricium.Data_Comment{
			Category:  "Foo",
			Message:   "Bar",
			StartLine: 5 + numHeaderLines,
			EndLine:   5 + numHeaderLines,
			StartChar: 2,
			EndChar:   4,
			Suggestions: []*tricium.Data_Suggestion{
				{
					Replacements: []*tricium.Data_Replacement{
						{
							StartLine: 5 + numHeaderLines,
							EndLine:   5 + numHeaderLines,
							StartChar: 2,
							EndChar:   4,
						},
					},
				},
			},
		}))
	})

	ftt.Run("File level comments don't have line numbers adjusted", t, func(t *ftt.Test) {
		comment := &tricium.Data_Comment{
			Category: "Foo",
			Message:  "Bar",
		}
		adjustCommitMessageComment(comment)
		assert.Loosely(t, comment, should.Resemble(&tricium.Data_Comment{
			Category: "Foo",
			Message:  "Bar",
		}))
	})
}
