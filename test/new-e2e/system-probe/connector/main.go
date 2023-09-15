package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/scylladb/go-sshtools"
	"golang.org/x/crypto/ssh"
)

const (
	FailConfig  = "config_fail"
	FailConnect = "connect_fail"
	FailStart   = "start_fail"
	FailWait    = "wait_fail"
	Success     = "success"
	Fail        = "fail"
)

var metrics = map[string]string{
	"gitlab": "connector_gitlab",
	"vm":     "connector_vm",
}

type Args struct {
	host                    string
	user                    string
	port                    int
	serverKeepAliveInterval time.Duration
	serverKeepAliveMaxCount int
	sshFilePath             string
	vmCommand               string
}

func readArgs() *Args {
	userPtr := flag.String("user", "", "SSH user")
	hostPtr := flag.String("host", "", "Host ip to connect to")
	portPtr := flag.Int("port", 22, "Port for ssh server")
	serverAlivePtr := flag.Int("server-alive-interval", 5, "Interval at which to send keep alive messages")
	serverAliveCountPtr := flag.Int("server-alive-count", 560, "Maximum keep alive messages to send before disconnecting upon no reply")
	sshFilePathPtr := flag.String("ssh-file", "", "Path to private ssh key")
	vmCmd := flag.String("vm-cmd", "", "command to run on VM")

	flag.Parse()

	return &Args{
		host:                    *hostPtr,
		user:                    *userPtr,
		port:                    *portPtr,
		serverKeepAliveInterval: time.Duration(*serverAlivePtr) * time.Second,
		serverKeepAliveMaxCount: *serverAliveCountPtr,
		sshFilePath:             *sshFilePathPtr,
		vmCommand:               *vmCmd,
	}
}

type ConnectorInfo struct {
	// For gitlab runner this will be the job id
	// For metal instance this will be empty
	connectorHost string
	connectorType string
}

func getConnectorInfo() (ConnectorInfo, error) {
	connectorType := "vm"
	if _, ok := os.LookupEnv("GITLAB_CI"); ok {
		connectorType = "gitlab"
	}

	if _, ok := metrics[connectorType]; !ok {
		return ConnectorInfo{}, fmt.Errorf("unknown connector type: %s", connectorType)
	}

	return ConnectorInfo{
		connectorHost: os.Getenv("CI_JOB_ID"),
		connectorType: connectorType,
	}, nil
}

func sshCommunicator(args *Args, sshKey []byte) (*sshtools.Communicator, error) {
	config := sshtools.Config{
		Port:                args.port,
		ServerAliveInterval: args.serverKeepAliveInterval,
		ServerAliveCountMax: args.serverKeepAliveMaxCount,
	}
	config, err := config.WithIdentityFileAuth(args.user, sshKey)
	if err != nil {
		return nil, fmt.Errorf("unable to build sshtools config: %w", err)
	}
	config.HostKeyCallback = ssh.InsecureIgnoreHostKey()

	return sshtools.NewCommunicator(args.host, config, sshtools.ContextDialer(&net.Dialer{}), log.New(os.Stdout, "", log.LstdFlags)), nil
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() (err error) {
	args := readArgs()
	cinfo, err := getConnectorInfo()
	if err != nil {
		return fmt.Errorf("get connector info: %s", err)
	}
	var cmd sshtools.Cmd
	key, err := os.ReadFile(args.sshFilePath)
	if err != nil {
		return fmt.Errorf("read private key: %s", err)
	}

	var failType string
	status := Fail
	defer func() {
		if serr := submitExecutionMetric(cinfo, failType, status); serr != nil {
			err = serr
		}
	}()

	communicator, err := sshCommunicator(args, key)
	if err != nil {
		failType = FailConfig
		return fmt.Errorf("communicator: %s", err)
	}

	ctx := context.Background()
	if err := communicator.Connect(ctx); err != nil {
		failType = FailConnect
		return fmt.Errorf("connect: %s", err)
	}

	cmd.Command = args.vmCommand
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := communicator.Start(ctx, &cmd); err != nil {
		failType = FailStart
		return fmt.Errorf("communicator start: %s", err)
	}

	if err := cmd.Wait(); err != nil {
		failType = FailWait
		return fmt.Errorf("wait: %s", err)
	}

	status = Success
	return nil
}

func buildMetric(cinfo ConnectorInfo, failType, status string) datadogV2.MetricPayload {
	tags := []string{
		fmt.Sprintf("status:%s", status),
	}
	if failType != "" {
		tags = append(tags, fmt.Sprintf("error:%s", failType))
	}
	if cinfo.connectorHost {
		tags = append(tags, fmt.Sprintf("host:%s", cinfo.connectorHost))
	}
	return datadogV2.MetricPayload{
		Series: []datadogV2.MetricSeries{
			{
				Metric: metrics[cinfo.connectorType],
				Type:   datadogV2.METRICINTAKETYPE_COUNT.Ptr(),
				Points: []datadogV2.MetricPoint{
					{
						Timestamp: datadog.PtrInt64(time.Now().Unix()),
						Value:     datadog.PtrFloat64(1),
					},
				},
				Tags: tags,
			},
		},
	}
}

func submitExecutionMetric(cinfo ConnectorInfo, failType, status string) error {
	if _, ok := os.LookupEnv("DD_API_KEY"); !ok {
		fmt.Fprintf(os.Stderr, "skipping sending metric because DD_API_KEY not present")
		return nil
	}

	metricBody := buildMetric(cinfo, failType, status)

	ctx := datadog.NewDefaultContext(context.Background())
	configuration := datadog.NewConfiguration()
	apiClient := datadog.NewAPIClient(configuration)
	api := datadogV2.NewMetricsApi(apiClient)
	resp, r, err := api.SubmitMetrics(ctx, metricBody, *datadogV2.NewSubmitMetricsOptionalParameters())

	if err != nil {
		fmt.Fprintf(os.Stderr, "Full HTTP response: %v\n", r)
		return fmt.Errorf("error when calling `MetricsApi.SubmitMetrics`: %v\n", err)
	}

	responseContent, _ := json.MarshalIndent(resp, "", "  ")
	fmt.Fprintf(os.Stdout, "Response from `MetricsApi.SubmitMetrics`:\n%s\n", responseContent)

	return nil
}
