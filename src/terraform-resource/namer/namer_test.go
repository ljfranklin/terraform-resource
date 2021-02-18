package namer_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/ljfranklin/terraform-resource/namer"
)

var _ = Describe("Namer", func() {

	Describe("#RandomName", func() {

		It("generates a new random name with each call", func() {
			generator := namer.New()

			alreadyGeneratedNames := map[string]bool{}
			collisionSampleSize := 20
			for i := 0; i < collisionSampleSize; i++ {
				randomName := generator.RandomName()
				Expect(randomName).To(MatchRegexp("[a-z]+-[a-z]+"))
				Expect(alreadyGeneratedNames).ToNot(HaveKey(randomName),
					"Unexpected random naming collision occurred with sample size %d", collisionSampleSize)
				alreadyGeneratedNames[randomName] = true
			}
		})
	})
})
