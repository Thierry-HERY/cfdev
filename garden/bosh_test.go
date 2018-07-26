package garden_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	gdn "code.cloudfoundry.org/cfdev/garden"
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/garden/gardenfakes"
)

var _ = Describe("DeployBosh", func() {
	var (
		fakeClient *gardenfakes.FakeClient
		err        error
		gclient    *gdn.Garden
	)

	BeforeEach(func() {
		fakeClient = new(gardenfakes.FakeClient)
		fakeClient.CreateReturns(nil, errors.New("some error"))
		gclient = &gdn.Garden{Client: fakeClient}
	})

	JustBeforeEach(func() {
		err = gclient.DeployBosh()
	})

	It("creates a container", func() {
		Expect(fakeClient.CreateCallCount()).To(Equal(1))
		spec := fakeClient.CreateArgsForCall(0)

		Expect(spec).To(Equal(garden.ContainerSpec{
			Handle:     "deploy-bosh",
			Privileged: true,
			Network:    "10.246.0.0/16",
			Image: garden.ImageRef{
				URI: "/var/vcap/cache/workspace.tar",
			},
			BindMounts: []garden.BindMount{
				{
					SrcPath: "/var/vcap",
					DstPath: "/var/vcap",
					Mode:    garden.BindMountModeRW,
				},
				{
					SrcPath: "/var/vcap/cache",
					DstPath: "/var/vcap/cache",
					Mode:    garden.BindMountModeRO,
				},
			},
		}))
	})

	Context("creating the container succeeds", func() {
		var (
			fakeContainer *gardenfakes.FakeContainer
		)

		BeforeEach(func() {
			fakeContainer = new(gardenfakes.FakeContainer)
			fakeContainer.RunReturns(nil, errors.New("some error"))
			fakeClient.CreateReturns(fakeContainer, nil)
		})

		It("starts to deploy bosh", func() {
			Expect(fakeContainer.RunCallCount()).To(Equal(1))

			spec, _ := fakeContainer.RunArgsForCall(0)
			Expect(spec).To(Equal(garden.ProcessSpec{
				ID:   "deploy-bosh",
				Path: "/bin/bash",
				Args: []string{"/var/vcap/cache/bin/deploy-bosh"},
				User: "root",
			}))
		})

		Context("when deploying bosh succeeds", func() {
			BeforeEach(func() {
				process := new(gardenfakes.FakeProcess)
				process.WaitReturns(0, nil)
				fakeContainer.RunReturns(process, nil)
			})

			It("returns without an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("deletes the container", func() {
				Expect(fakeClient.DestroyCallCount()).To(Equal(1))
				handle := fakeClient.DestroyArgsForCall(0)
				Expect(handle).To(Equal("deploy-bosh"))
			})
		})

		Context("when the deploy cannot start", func() {
			BeforeEach(func() {
				fakeContainer.RunReturns(nil, errors.New("unable to start process"))
			})

			It("returns an error", func() {
				Expect(err).To(MatchError("unable to start process"))
			})
		})

		Context("when the deploy finishes with a non-zero exit code", func() {
			BeforeEach(func() {
				process := new(gardenfakes.FakeProcess)
				process.WaitReturns(23, nil)
				fakeContainer.RunReturns(process, nil)
			})

			It("returns an error", func() {
				Expect(err).To(MatchError("process exited with status 23"))
			})
		})

		Context("when we cannot determine the state of the deploy", func() {
			BeforeEach(func() {
				process := new(gardenfakes.FakeProcess)
				process.WaitReturns(-10, errors.New("connection to garden lost"))
				fakeContainer.RunReturns(process, nil)
			})

			It("returns an error", func() {
				Expect(err).To(MatchError("connection to garden lost"))
			})
		})
	})

	Context("creating the container fails", func() {
		BeforeEach(func() {
			fakeClient.CreateReturns(nil, errors.New("unable to create container"))
		})

		It("forwards the error", func() {
			Expect(err).To(MatchError("unable to create container"))
		})
	})
})
