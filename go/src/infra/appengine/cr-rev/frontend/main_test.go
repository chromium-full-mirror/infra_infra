package main

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestGetGerritUrl(t *testing.T) {
	Convey("returns CL", t, func() {
		url := getGerritUrl("https://www.example.com/", "42")
		So(url, ShouldEqual, "https://www.example.com/c/42")
	})

	Convey("returns dashboard", t, func() {
		url := getGerritUrl("https://www.example.com/", "")
		So(url, ShouldEqual, "https://www.example.com/dashboard/self")
	})
}
