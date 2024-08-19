package utils

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"

	ca "infra/libs/fleet/protos"
	fleet "infra/libs/fleet/protos/go"
)

func TestSanitizeChopsAsset(t *testing.T) {
	asset1 := &ca.ChopsAsset{
		Id: " Eddie the Computer ",
		Location: &fleet.Location{
			Lab:      "Heart Of Gold ",
			Aisle:    " Starboard Aisle 6",
			Row:      " 10 ",
			Rack:     " 1",
			Position: "SomewhereImprobable ",
		},
	}

	asset2 := &ca.ChopsAsset{
		Id: "Marvin ",
	}

	Convey("Test Sanitizing", t, func() {
		asset1New := SanitizeChopsAsset([]*ca.ChopsAsset{asset1})
		So(asset1New[0].Id, ShouldEqual, "Eddie the Computer")
		So(asset1New[0].Location.Lab, ShouldEqual, "Heart Of Gold")
		So(asset1New[0].Location.Aisle, ShouldEqual, "Starboard Aisle 6")
		So(asset1New[0].Location.Row, ShouldEqual, "10")
		So(asset1New[0].Location.Rack, ShouldEqual, "1")
		So(asset1New[0].Location.Position, ShouldEqual, "SomewhereImprobable")
		asset2New := SanitizeChopsAsset([]*ca.ChopsAsset{asset2})
		So(asset2New[0].Id, ShouldEqual, "Marvin")
	})

}
