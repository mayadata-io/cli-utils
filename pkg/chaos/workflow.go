package chaos

import (
	"fmt"
	"github.com/go-resty/resty/v2"
	ymlparser "gopkg.in/yaml.v2"
	"log"
	"net/url"
)

type WorkflowYAML struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name      string            `yaml:"name"`
		Namespace string            `yaml:"namespace"`
		Labels    map[string]string `yaml:"labels"`
	} `yaml:"metadata"`
	Spec struct {
		Arguments       Arguments `yaml:"arguments"`
		Entrypoint      string    `yaml:"entrypoint"`
		SecurityContext struct {
			RunAsNonRoot bool `yaml:"runAsNonRoot"`
			RunAsUser    int  `yaml:"runAsUser"`
		} `yaml:"securityContext"`
		ServiceAccountName string     `yaml:"serviceAccountName"`
		Templates          []Template `yaml:"templates"`
	} `yaml:"spec"`
}

type Input struct {
	Artifacts []Artifact `yaml:"artifacts"`
}

type Artifact struct {
	Name string `yaml:"name"`
	Path string `yaml:"path"`
	Raw  Raw    `yaml:"raw"`
}

type Raw struct {
	Data string `yaml:"data"`
}

type Template struct {
	Name      string   `yaml:"name"`
	Steps     [][]Step `yaml:"steps,omitempty"`
	Input     Input    `yaml:"inputs,omitempty"`
	Container struct {
		Args    []string `yaml:"args"`
		Command []string `yaml:"command"`
		Image   string   `yaml:"image"`
	} `yaml:"container,omitempty"`
}

type Step struct {
	Name     string `yaml:"name"`
	Template string `yaml:"template"`
}

type Arguments struct {
	Parameters []Parameter `yaml:"parameters"`
}

type Parameter struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

type ListPkgData struct {
	Data struct {
		ListHubPkgData []PackageData `json:"ListHubPkgData"`
	} `json:"data"`
}
type PackageData struct {
	Experiments []string `json:"Experiments"`
	ChartName   string   `json:"chartName"`
}

type YAMLData struct {
	Data struct {
		GetYAMLData string `json:"getYAMLData"`
	} `json:"data"`
}

type GenerateWorkflowInputs struct {
	HubName        string
	ProjectID      string
	ChartName      string
	ExperimentName *string
	AccessToken    string
	FileType       *string
	URL            *url.URL
	WorkName       string
	WorkNamespace  string
	ClusterID      string
	Packages       []*PackageData
}

type GetClusters struct {
	Data struct {
		GetCluster []struct {
			ClusterID   string `json:"cluster_id"`
			ClusterName string `json:"cluster_name"`
		} `json:"getCluster"`
	} `json:"data"`
}

type GetHubStatus struct {
	Data struct {
		GetHubStatus []struct {
			ID      string `json:"id"`
			HubName string `json:"HubName"`
		} `json:"getHubStatus"`
	} `json:"data"`
}

func GetYamlData(inputs GenerateWorkflowInputs) (YAMLData, error) {
	client := resty.New()

	var yamlDataResponse YAMLData
	gql_query := `{"query":"query {\n  getYAMLData(experimentInput: {\n    ProjectID: \"` + inputs.ProjectID + `\"\n    HubName: \"` + inputs.HubName + `\"\n    ChartName: \"` + inputs.ChartName + `\"\n    ExperimentName: \"` + *inputs.ExperimentName + `\"\n    FileType: \"` + *inputs.FileType + `\"\n    \n  })\n}"}`
	response, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Authorization", fmt.Sprintf("%s", inputs.AccessToken)).
		SetHeader("Accept-Encoding", "gzip, deflate, br").
		SetBody(gql_query).
		SetResult(&yamlDataResponse).
		Post(
			fmt.Sprintf(
				"%s/%s/api/graphql/query",
				inputs.URL,
				"chaos",
			),
		)

	if err != nil || !response.IsSuccess() {
		return YAMLData{}, err
	}

	return yamlDataResponse, nil
}

func GenerateWorkflow(wf_inputs GenerateWorkflowInputs) ([]byte, error) {

	var yaml WorkflowYAML

	yaml.APIVersion = "argoproj.io/v1alpha1"
	yaml.Kind = "Workflow"
	yaml.Metadata.Name = wf_inputs.WorkName
	yaml.Metadata.Namespace = wf_inputs.WorkNamespace
	yaml.Metadata.Labels = map[string]string{
		"cluster_id": wf_inputs.ClusterID,
	}
	var pram Parameter
	pram.Name = "adminModeNamespace"
	pram.Value = wf_inputs.WorkNamespace
	yaml.Spec.Arguments.Parameters = append(yaml.Spec.Arguments.Parameters, pram)

	yaml.Spec.Entrypoint = "custom-chaos"
	yaml.Spec.SecurityContext.RunAsNonRoot = true
	yaml.Spec.SecurityContext.RunAsUser = 1000

	var (
		custom_chaos        Template
		install_experiments Template
		engines             []Template
		revert_chaos        Template
	)

	custom_chaos.Name = "custom-chaos"
	custom_chaos.Steps = append(custom_chaos.Steps, []Step{{Name: "install-chaos-experiments", Template: "install-chaos-experiments"}})

	install_experiments.Name = "install-chaos-experiments"
	install_experiments.Container.Image = "lachlanevenson/k8s-kubectl"
	install_experiments.Container.Command = []string{"sh", "-c"}
	install_experiments.Container.Args = []string{""}

	revert_chaos.Name = "revert-chaos"
	revert_chaos.Container.Image = "lachlanevenson/k8s-kubectl"
	revert_chaos.Container.Command = []string{"sh", "-c"}
	revert_chaos.Container.Args = []string{"kubectl delete chaosengine "}

	for _, pkg := range wf_inputs.Packages {

		for _, experiment := range pkg.Experiments {

			wf_inputs.ExperimentName = &experiment

			custom_chaos.Steps = append(custom_chaos.Steps, []Step{{Name: experiment, Template: experiment}})
			var file_type = "experiment"
			wf_inputs.FileType = &file_type
			yamlData, err := GetYamlData(wf_inputs)
			if err != nil {
				log.Print(err)
			}

			install_experiments.Input.Artifacts = append(install_experiments.Input.Artifacts,
				Artifact{
					Name: experiment,
					Path: "/tmp/" + experiment + ".yaml",

					Raw: Raw{
						Data: fmt.Sprint(yamlData.Data.GetYAMLData),
					},
				})

			install_experiments.Container.Args[0] += "kubectl apply -f /tmp/" + experiment + ".yaml" + "-n {{workflow.parameters.adminModeNamespace}} | "

			revert_chaos.Container.Args[0] += experiment + " "

			file_type = "engine"
			wf_inputs.FileType = &file_type

			yamlData, err = GetYamlData(wf_inputs)
			if err != nil {
				log.Print(err)
			}

			var engine Template
			engine.Name = experiment
			engine.Container.Args = append(engine.Container.Args, "-file=/tmp/chaosengine-"+experiment+".yaml")
			engine.Container.Args = append(engine.Container.Args, "-saveName=/tmp/engine-name")
			engine.Container.Image = "litmuschaos/litmus-checker:latest"
			engine.Input.Artifacts = append(engine.Input.Artifacts, Artifact{
				Name: experiment,
				Path: "/tmp/chaosengine-" + experiment + ".yaml",
				Raw:  Raw{Data: fmt.Sprintln(yamlData.Data.GetYAMLData)},
			})

			engines = append(engines, engine)
		}
	}

	// Custom chaos
	custom_chaos.Steps = append(custom_chaos.Steps, []Step{{Name: "revert-chaos", Template: "revert-chaos"}})
	yaml.Spec.Templates = append(yaml.Spec.Templates, custom_chaos)

	// Install experiments
	install_experiments.Container.Args[0] += "sleep 30"
	yaml.Spec.Templates = append(yaml.Spec.Templates, install_experiments)

	// Install engines
	yaml.Spec.Templates = append(yaml.Spec.Templates, engines...)

	// Revert Chaos
	revert_chaos.Container.Args[0] += "-n {{workflow.parameters.adminModeNamespace}}"
	yaml.Spec.Templates = append(yaml.Spec.Templates, revert_chaos)

	yamlByte, err := ymlparser.Marshal(yaml)
	if err != nil {
		return nil, err
	}

	return yamlByte, nil
}

func GetClustersQuery(project_id string, access_token string, url *url.URL) (GetClusters, error) {
	client := resty.New()

	var getClusters GetClusters
	gql_query := `{"query":"query {\n  getCluster(project_id: \"` + project_id + `\"){\n    cluster_id\n cluster_name\n  }\n}"}`
	resp, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Authorization", fmt.Sprintf("%s", access_token)).
		SetHeader("Accept-Encoding", "gzip, deflate, br").
		SetBody(gql_query).
		// SetResult automatic unmarshalling for the request,
		// if response status code is between 200 and 299
		SetResult(&getClusters).
		Post(
			fmt.Sprintf(
				"%s/%s/api/graphql/query",
				url,
				"chaos",
			),
		)
	if err != nil || !resp.IsSuccess() {
		return GetClusters{}, err
	}

	return getClusters, nil
}

func GetHubStatusQuery(project_id string, access_token string, url *url.URL) (GetHubStatus, error) {
	client := resty.New()

	var getHubStatus GetHubStatus
	gql_query := `{"query":"query {\n  getHubStatus(projectID: \"` + project_id + `\"){\n    id\n HubName \n  }\n}"}`
	response, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Authorization", fmt.Sprintf("%s", access_token)).
		SetHeader("Accept-Encoding", "gzip, deflate, br").
		SetBody(gql_query).
		// SetResult automatic unmarshalling for the request,
		// if response status code is between 200 and 299
		SetResult(&getHubStatus).
		Post(
			fmt.Sprintf(
				"%s/%s/api/graphql/query",
				url,
				"chaos",
			),
		)
	if err != nil || !response.IsSuccess() {
		return GetHubStatus{}, nil
	}

	return getHubStatus, nil
}

func ListPkgDataQuery(project_id string, hub_id string, access_token string, url *url.URL) (ListPkgData, error) {
	var pkgdata ListPkgData

	client := resty.New()

	gql_query := `{"query":"query {\n  ListHubPkgData(projectID: \"` + project_id + `\", hubID: \"` + hub_id + `\"){\n    Experiments\n    chartName\n  }\n}"}`
	response, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Authorization", fmt.Sprintf("%s", access_token)).
		SetHeader("Accept-Encoding", "gzip, deflate, br").
		SetBody(gql_query).
		// SetResult automatic unmarshalling for the request,
		// if response status code is between 200 and 299
		SetResult(&pkgdata).
		Post(
			fmt.Sprintf(
				"%s/%s/api/graphql/query",
				url,
				"chaos",
			),
		)
	if err != nil || !response.IsSuccess() {
		log.Print(err)
	}

	return pkgdata, nil
}
