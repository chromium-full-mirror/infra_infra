The expect package was copied from https://github.com/google/goexpect at ab937bf
as the project was archived and not maintained and there was no maintained
alternative.

It was required by the ccd package (infra/go/src/infra/cros/satlab/satlab/internal/ccd).

Changes made to the original code: removed non-inclusive terminology, formatted
the code with gofmt, fixed typos.
