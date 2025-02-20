// Copyright 2017 The LUCI Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package spec

import (
	"testing"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"

	"go.chromium.org/infra/tools/vpython/legacy/vpython2.7/luci/api/vpython"
)

func TestNormalizeAndHash(t *testing.T) {
	t.Parallel()

	ftt.Run(`Test manifest generation`, t, func(t *ftt.Test) {
		otherTag := &vpython.PEP425Tag{Python: "otherPython", Abi: "otherABI", Platform: "otherPlatform"}
		maybeTag := &vpython.PEP425Tag{Python: "maybePython", Abi: "maybeABI", Platform: "maybePlatform"}

		pkgFoo := &vpython.Spec_Package{Name: "foo", Version: "1"}
		pkgFooV2 := &vpython.Spec_Package{Name: "foo", Version: "2"}
		pkgBar := &vpython.Spec_Package{Name: "bar", Version: "2"}
		pkgBaz := &vpython.Spec_Package{Name: "baz", Version: "3"}

		env := vpython.Environment{
			Pep425Tag: []*vpython.PEP425Tag{otherTag},
		}
		var rt vpython.Runtime

		t.Run(`Will normalize an empty spec`, func(t *ftt.Test) {
			assert.Loosely(t, NormalizeEnvironment(&env), should.BeNil)
			assert.Loosely(t, env, should.Resemble(vpython.Environment{
				Spec:      &vpython.Spec{},
				Runtime:   &vpython.Runtime{},
				Pep425Tag: []*vpython.PEP425Tag{otherTag},
			}))
		})

		t.Run(`With a non-nil spec`, func(t *ftt.Test) {
			env.Spec = &vpython.Spec{}

			t.Run(`Will normalize to sorted order.`, func(t *ftt.Test) {
				env.Spec.Wheel = []*vpython.Spec_Package{pkgFoo, pkgBar, pkgBaz}
				assert.Loosely(t, NormalizeEnvironment(&env), should.BeNil)
				assert.Loosely(t, env.Spec, should.Resemble(&vpython.Spec{
					Wheel: []*vpython.Spec_Package{pkgBar, pkgBaz, pkgFoo},
				}))

				assert.Loosely(t, Hash(env.Spec, &rt), should.Equal("1e32c02610b51f8c3807203fccd3e8d01d252868d52eb4ee9df135ef6533c5ae"))
				assert.Loosely(t, Hash(env.Spec, &rt, "extra"), should.Equal("d047eb021f50534c050aaa10c70dc7b4a9b511fab00cf67a191b2b0805f24420"))
			})

			t.Run(`With a match entry, will match tags`, func(t *ftt.Test) {
				pkgMaybe := &vpython.Spec_Package{Name: "maybe", Version: "3", MatchTag: []*vpython.PEP425Tag{
					{Python: maybeTag.Python},
				}}
				env.Spec.Wheel = []*vpython.Spec_Package{pkgFoo, pkgMaybe}

				t.Run(`Will omit the package if it doesn't match a tag`, func(t *ftt.Test) {
					assert.Loosely(t, NormalizeEnvironment(&env), should.BeNil)
					assert.Loosely(t, env.Spec, should.Resemble(&vpython.Spec{
						Wheel: []*vpython.Spec_Package{pkgFoo},
					}))
				})

				t.Run(`Will include the package if it matches a tag, and strip the match field.`, func(t *ftt.Test) {
					env.Pep425Tag = append(env.Pep425Tag, maybeTag)

					assert.Loosely(t, NormalizeEnvironment(&env), should.BeNil)

					pkgMaybe.MatchTag = nil
					assert.Loosely(t, env.Spec, should.Resemble(&vpython.Spec{
						Wheel: []*vpython.Spec_Package{pkgFoo, pkgMaybe},
					}))
				})
			})

			t.Run(`With multiple match entries, will match tags`, func(t *ftt.Test) {
				pkgMaybe := &vpython.Spec_Package{Name: "maybe", Version: "3", MatchTag: []*vpython.PEP425Tag{
					{Python: maybeTag.Python},
				}}
				pkgMaybeNonexistTag := &vpython.Spec_Package{Name: "maybe", Version: "3", MatchTag: []*vpython.PEP425Tag{
					{Python: "nonexist"},
				}}
				env.Spec.Wheel = []*vpython.Spec_Package{pkgMaybe, pkgFoo, pkgMaybeNonexistTag}
				env.Pep425Tag = append(env.Pep425Tag, maybeTag)

				assert.Loosely(t, NormalizeEnvironment(&env), should.BeNil)

				pkgMaybe.MatchTag = nil
				assert.Loosely(t, env.Spec, should.Resemble(&vpython.Spec{
					Wheel: []*vpython.Spec_Package{pkgFoo, pkgMaybe},
				}))
			})

			t.Run(`With one absolute and one match, will always match`, func(t *ftt.Test) {
				pkgAlways := &vpython.Spec_Package{Name: "maybe", Version: "3"}
				pkgMaybeNonexistTag := &vpython.Spec_Package{Name: "maybe", Version: "3", MatchTag: []*vpython.PEP425Tag{
					{Python: "nonexist"},
				}}
				env.Spec.Wheel = []*vpython.Spec_Package{pkgFoo, pkgMaybeNonexistTag}
				env.Pep425Tag = append(env.Pep425Tag, maybeTag)

				assert.Loosely(t, NormalizeEnvironment(&env), should.BeNil)
				assert.Loosely(t, env.Spec, should.Resemble(&vpython.Spec{
					Wheel: []*vpython.Spec_Package{pkgFoo},
				}))

				env.Spec.Wheel = []*vpython.Spec_Package{pkgAlways, pkgFoo, pkgMaybeNonexistTag}

				assert.Loosely(t, NormalizeEnvironment(&env), should.BeNil)
				assert.Loosely(t, env.Spec, should.Resemble(&vpython.Spec{
					Wheel: []*vpython.Spec_Package{pkgFoo, pkgAlways},
				}))
			})

			t.Run(`Will normalize if there are duplicate wheels that share a version.`, func(t *ftt.Test) {
				env.Spec.Wheel = []*vpython.Spec_Package{pkgFoo, pkgFoo, pkgBar, pkgBaz}
				assert.Loosely(t, NormalizeEnvironment(&env), should.BeNil)
				assert.Loosely(t, env.Spec, should.Resemble(&vpython.Spec{
					Wheel: []*vpython.Spec_Package{pkgBar, pkgBaz, pkgFoo},
				}))
			})

			t.Run(`Will fail to normalize if there are duplicate wheels with different versions.`, func(t *ftt.Test) {
				env.Spec.Wheel = []*vpython.Spec_Package{pkgFoo, pkgFooV2, pkgFoo, pkgBar, pkgBaz}
				assert.Loosely(t, NormalizeEnvironment(&env), should.ErrLike("multiple versions for package"))
			})

			t.Run(`Will normalize if there is a duplicate wheel with a different version, but it doesn't match.`, func(t *ftt.Test) {
				env.Pep425Tag = append(env.Pep425Tag, maybeTag)

				pkgMaybe := &vpython.Spec_Package{Name: "maybe", Version: "3", MatchTag: []*vpython.PEP425Tag{
					{Python: maybeTag.Python},
				}}

				pkgSkipped := &vpython.Spec_Package{Name: "maybe", Version: "4", NotMatchTag: []*vpython.PEP425Tag{
					{Python: otherTag.Python},
				}}

				env.Spec.Wheel = []*vpython.Spec_Package{pkgMaybe, pkgSkipped}
				assert.Loosely(t, NormalizeEnvironment(&env), should.BeNil)
			})
		})
	})
}
