package client

import (
	"fmt"
	"net/url"
	"dvm/engine"
)


func (cli *DvmClient) DvmCmdStop(args ...string) error {
	if len(args) == 0 {
		return fmt.Errorf("Can not accept the 'stop' command without pod name!")
	}
	podName := args[0]

	v := url.Values{}
	v.Set("podName", podName)
	body, _, err := readBody(cli.call("GET", "/stop?"+v.Encode(), nil, nil));
	if err != nil {
		return err
	}
	out := engine.NewOutput()
	remoteInfo, err := out.AddEnv()
	if err != nil {
		return err
	}

	if _, err := out.Write(body); err != nil {
		fmt.Printf("Error reading remote info: %s", err)
		return err
	}
	out.Close()
	// This 'ID' stands for pod name
	// This 'Code' should be E_SHUTDOWN
	// THis 'Cause' ..
	if remoteInfo.Exists("ID") {
		// TODO ...
	}

	fmt.Printf("Success to shutdown the POD: %s!\n", podName)
	return nil
}