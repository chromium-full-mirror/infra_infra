package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	gerrit "github.com/andygrunwald/go-gerrit"

	"go.chromium.org/luci/appengine/gaetesting"
	"go.chromium.org/luci/common/logging"
	"go.chromium.org/luci/common/logging/gologger"
	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
	"go.chromium.org/luci/server/auth"
	"go.chromium.org/luci/server/auth/authtest"
)

func TestGerritClient(t *testing.T) {
	testMux := http.NewServeMux()
	testServer := httptest.NewServer(testMux)

	ftt.Run("get client", t, func(t *ftt.Test) {
		c := gaetesting.TestingContext()
		c = authtest.MockAuthConfig(c)
		authState := &authtest.FakeState{
			Identity: "user:user@example.com",
		}
		c = auth.WithState(c, authState)
		c = withGerritInstance(c, testServer.URL)
		client, err := getGerritClient(c)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, client, should.NotBeNil)
	})

	ftt.Run("create CL", t, func(t *ftt.Test) {
		c := gaetesting.TestingContext()
		c = gologger.StdConfig.Use(c)
		c = authtest.MockAuthConfig(c)
		authState := &authtest.FakeState{
			Identity: "user:user@example.com",
		}
		c = auth.WithState(c, authState)

		created, uploaded, published := false, false, false

		testMux.HandleFunc("/a/changes/", func(w http.ResponseWriter, r *http.Request) {
			logging.Infof(c, "%s: %v", r.Method, r.URL.Path)
			switch r.Method {
			case "POST":
				if strings.HasSuffix(r.URL.Path, ":publish") {
					published = true
					break
				}
				marshalled, _ := json.Marshal(&gerrit.ChangeInfo{
					Project:  "project",
					Branch:   "branch",
					Subject:  "subject",
					Status:   "DRAFT",
					Topic:    "",
					ChangeID: "chromium~whatever1234",
					ID:       "1234",
				})
				fmt.Fprintf(w, ")]}'\n%s", string(marshalled))
				created = true
			case "PUT":
				w.WriteHeader(http.StatusOK)
				uploaded = true
			default:
				w.WriteHeader(http.StatusBadRequest)
			}
		})

		c = withGerritInstance(c, testServer.URL)
		client, err := getGerritClient(c)
		assert.Loosely(t, err, should.BeNil)

		fileContents := map[string]string{
			"test_file.txt": "file contents",
		}
		changeID, err := createCL(client, "project", "branch", "subject", fileContents)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, created, should.BeTrue)
		assert.Loosely(t, uploaded, should.BeTrue)
		assert.Loosely(t, published, should.BeTrue)
		assert.Loosely(t, changeID, should.Equal("1234"))
	})
}
