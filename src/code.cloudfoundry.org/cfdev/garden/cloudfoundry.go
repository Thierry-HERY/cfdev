package garden

import (
	"fmt"
	"os"

	"code.cloudfoundry.org/cfdev/errors"
	"code.cloudfoundry.org/garden"
	"gopkg.in/yaml.v2"
)

func DeployCloudFoundry(client garden.Client, dockerRegistries []string) error {
	containerSpec := garden.ContainerSpec{
		Handle:     "deploy-cf",
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
	}

	if len(dockerRegistries) > 0 {
		bytes, err := yaml.Marshal(dockerRegistries)

		if err != nil {
			return err
		}

		containerSpec.Env = append(containerSpec.Env, "DOCKER_REGISTRIES="+string(bytes))
	}

	container, err := client.Create(containerSpec)
	if err != nil {
		return err
	}

	out := singleLineReporter(os.Stdout)
	process, err := container.Run(garden.ProcessSpec{
		ID:   "deploy-cf",
		Path: "/usr/bin/deploy-cf",
		User: "root",
	}, garden.ProcessIO{
		Stdout: out,
	})

	if err != nil {
		return err
	}

	exitCode, err := process.Wait()
	if err != nil {
		return err
	}

	out.Close()

	if exitCode != 0 {
		return errors.SafeWrap(nil, fmt.Sprintf("process exited with status %d", exitCode))
	}

	client.Destroy("deploy-cf")

	return nil
}
