package k8s

import (
	"fmt"
	"github.com/mayadata-io/cli-utils/pkg/common"
	"os/exec"
)

func ApplyYaml(token string, cred common.Credentials, yamlPath string) (output string, err error) {
	path := fmt.Sprintf("%s/%s/%s.yaml", cred.Host, yamlPath, token)
	args := []string{"kubectl", "apply", "-f", path}
	stdout, err := exec.Command(args[0], args[1:]...).CombinedOutput()
	if err != nil {
		err = fmt.Errorf("Error: %v", err)
	}
	return string(stdout), err
}
