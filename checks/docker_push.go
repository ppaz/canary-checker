package checks

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"github.com/flanksource/kommons"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
)

type DockerPushChecker struct {
	kommons *kommons.Client `yaml:"-" json:"-"`
}

func (c *DockerPushChecker) SetClient(client *kommons.Client) {
	c.kommons = client
}

func (c *DockerPushChecker) Run(config v1.CanarySpec) []*pkg.CheckResult {
	var results []*pkg.CheckResult
	for _, conf := range config.DockerPush {
		results = append(results, c.Check(conf))
	}
	return results
}

// Type: returns checker type
func (c *DockerPushChecker) Type() string {
	return "dockerPush"
}

// Run: Check every entry from config according to Checker interface
// Returns check result and metrics
func (c *DockerPushChecker) Check(extConfig external.Check) *pkg.CheckResult {
	check := extConfig.(v1.DockerPushCheck)
	start := time.Now()
	ctx := context.Background()
	var username, password string
	var err error
	namespace := check.GetNamespace()
	_, username, err = c.kommons.GetEnvValue(check.Auth.Username, namespace)
	if err != nil {
		return Failf(check, "failed to fetch username from envVar: %v", err)
	}
	_, password, err = c.kommons.GetEnvValue(check.Auth.Password, namespace)
	if err != nil {
		return Failf(check, "failed to fetch password from envVar: %v", err)
	}
	authConfig := types.AuthConfig{
		Username: username,
		Password: password,
	}
	encodedJSON, _ := json.Marshal(authConfig)
	authStr := base64.URLEncoding.EncodeToString(encodedJSON)
	out, err := dockerClient.ImagePush(ctx, check.Image, types.ImagePushOptions{RegistryAuth: authStr})
	if err != nil {
		return Failf(check, "Failed to push image: %s", err)
	}

	buf := new(bytes.Buffer)
	defer out.Close()
	_, _ = buf.ReadFrom(out)
	lines := strings.Split(buf.String(), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		decodedResponse := struct {
			Status string `json:"status"`
			Error  string `json:"error"`
		}{}
		err = json.Unmarshal([]byte(line), &decodedResponse)
		if err != nil {
			return Failf(check, "Invalid response: %v: %s", err, line)
		}
		if decodedResponse.Error != "" {
			return Failf(check, "Failed to push %v", decodedResponse.Error)
		}
	}

	return &pkg.CheckResult{
		Check:    check,
		Pass:     true,
		Duration: time.Since(start).Milliseconds(),
		Metrics:  []pkg.Metric{},
	}
}
