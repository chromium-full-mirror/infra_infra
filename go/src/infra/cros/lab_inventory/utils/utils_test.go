package utils

import (
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
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

	ftt.Run("Test Sanitizing", t, func(t *ftt.Test) {
		asset1New := SanitizeChopsAsset([]*ca.ChopsAsset{asset1})
		assert.Loosely(t, asset1New[0].Id, should.Equal("Eddie the Computer"))
		assert.Loosely(t, asset1New[0].Location.Lab, should.Equal("Heart Of Gold"))
		assert.Loosely(t, asset1New[0].Location.Aisle, should.Equal("Starboard Aisle 6"))
		assert.Loosely(t, asset1New[0].Location.Row, should.Equal("10"))
		assert.Loosely(t, asset1New[0].Location.Rack, should.Equal("1"))
		assert.Loosely(t, asset1New[0].Location.Position, should.Equal("SomewhereImprobable"))
		asset2New := SanitizeChopsAsset([]*ca.ChopsAsset{asset2})
		assert.Loosely(t, asset2New[0].Id, should.Equal("Marvin"))
	})

}
