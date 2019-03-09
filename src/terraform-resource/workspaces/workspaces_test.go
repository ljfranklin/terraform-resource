package workspaces_test

import (
	"errors"
	"terraform-resource/terraform"
	"terraform-resource/terraform/terraformfakes"
	"terraform-resource/workspaces"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Workspaces", func() {

	Describe("#LatestVersionForEnv", func() {
		var fakeTerraform *terraformfakes.FakeClient

		Context("when the given env does not exist", func() {
			BeforeEach(func() {
				fakeTerraform = &terraformfakes.FakeClient{}
				fakeTerraform.WorkspaceListReturns([]string{}, nil)
				fakeTerraform.InitWithBackendReturns(nil)
			})

			It("returns an empty Version", func() {
				spaces := workspaces.New(fakeTerraform)

				version, err := spaces.LatestVersionForEnv("missing-env")
				Expect(err).To(BeNil())
				Expect(version).To(Equal(terraform.StateVersion{}))
			})
		})

		Context("when the given env exists", func() {
			BeforeEach(func() {
				fakeTerraform = &terraformfakes.FakeClient{}
				fakeTerraform.WorkspaceListReturns([]string{"some-env"}, nil)
				fakeTerraform.CurrentStateVersionReturns(terraform.StateVersion{
					Serial:  7,
					Lineage: "aaaaa",
				}, nil)
			})

			It("returns a Version with the given serial number", func() {
				spaces := workspaces.New(fakeTerraform)

				version, err := spaces.LatestVersionForEnv("some-env")
				Expect(err).To(BeNil())
				Expect(version).To(Equal(terraform.StateVersion{
					Serial:  7,
					Lineage: "aaaaa",
				}))
			})
		})

		Context("when initializing fails", func() {
			BeforeEach(func() {
				fakeTerraform = &terraformfakes.FakeClient{}
				fakeTerraform.InitWithBackendReturns(errors.New("some-error"))
			})

			It("returns the error", func() {
				spaces := workspaces.New(fakeTerraform)

				_, err := spaces.LatestVersionForEnv("some-env")
				Expect(err).To(MatchError("some-error"))
			})
		})

		Context("when listing workspaces returns an error", func() {
			BeforeEach(func() {
				fakeTerraform = &terraformfakes.FakeClient{}
				fakeTerraform.WorkspaceListReturns(nil, errors.New("some-error"))
			})

			It("returns the error", func() {
				spaces := workspaces.New(fakeTerraform)

				_, err := spaces.LatestVersionForEnv("some-env")
				Expect(err).To(MatchError("some-error"))
			})
		})

		Context("when fetching serial returns an error", func() {
			BeforeEach(func() {
				fakeTerraform = &terraformfakes.FakeClient{}
				fakeTerraform.WorkspaceListReturns([]string{"some-env"}, nil)
				fakeTerraform.CurrentStateVersionReturns(terraform.StateVersion{}, errors.New("some-error"))
			})

			It("returns the error", func() {
				spaces := workspaces.New(fakeTerraform)

				_, err := spaces.LatestVersionForEnv("some-env")
				Expect(err).To(MatchError("some-error"))
			})
		})
	})
})
