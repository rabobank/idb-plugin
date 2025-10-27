package main

import (
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "os"

    "code.cloudfoundry.org/cli/plugin"
    "github.com/cloudfoundry/go-cfclient/v3/client"
    plugins "github.com/rabobank/cf-plugins"
    "gopkg.in/yaml.v3"
    "idb-plugin/cfg"
)

type IdInfo struct {
    Subject  string   `json:"subject" yaml:"Subject"`
    Issuer   string   `json:"issuer" yaml:"Issuer"`
    Audience []string `json:"audience" yaml:"Audience"`
}
type IdbPlugin struct{}

func (c *IdbPlugin) Execute(cliConnection plugins.CliConnection, args []string) {
    if args[0] == "CLI-MESSAGE-UNINSTALL" {
        return
    }

    switch args[0] {
    case cfg.ShowIdentityDetails:
        if e := showIdentityDetails(cliConnection, args[1:]); e != nil {
            fmt.Println(e)
            os.Exit(1)
        }
    default:
        fmt.Println("Unknown command: " + args[0])
        os.Exit(1)
    }
}

func showIdentityDetails(connection plugins.CliConnection, args []string) error {
    if len(args) == 0 {
        return fmt.Errorf("missing service instance name")
    }

    token, e := connection.AccessToken()
    if e != nil {
        return fmt.Errorf("could not get access token: %s", e)
    }

    space, e := connection.GetCurrentSpace()
    if e != nil {
        return fmt.Errorf("could not get current space: %s", e)
    }

    cf := connection.CfClient()
    si, e := cf.ServiceInstances.Single(context.Background(), &client.ServiceInstanceListOptions{
        Names:      client.Filter{Values: []string{args[0]}},
        SpaceGUIDs: client.Filter{Values: []string{space.Guid}},
    })
    if e != nil {
        return fmt.Errorf("could not get service instance %s in space %s: %s", args[0], space.Name, e)
    }

    _, offering, e := cf.ServicePlans.GetIncludeServicePlan(context.Background(), si.Relationships.ServicePlan.Data.GUID)
    if e != nil {
        return fmt.Errorf("could not get service offering: %s", e)
    }

    broker, e := cf.ServiceBrokers.Get(context.Background(), offering.Relationships.ServiceBroker.Data.GUID)
    if e != nil {
        return fmt.Errorf("could not find cf.identity broker: %s", e)
    }

    request, e := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/api/info/%s", broker.URL, si.GUID), nil)
    if e != nil {
        return e
    }
    request.Header.Set("Authorization", token)
    r, e := http.DefaultClient.Do(request)
    if e != nil {
        return e
    }

    bytes, e := io.ReadAll(r.Body)
    if e != nil {
        return e
    }
    if len(bytes) == 0 {
        return fmt.Errorf("no identity details provided")
    }

    details := &IdInfo{}
    if e = json.Unmarshal(bytes, &details); e != nil {
        return e
    }
    _ = yaml.NewEncoder(os.Stdout).Encode(details)
    return nil
}

func (c *IdbPlugin) GetMetadata() plugin.PluginMetadata {
    return cfg.Metadata
}

func main() {
    if len(os.Args) == 1 {
        _, _ = fmt.Fprintf(os.Stderr, "This executable is a cf plugin.\n"+
            "Run `cf install-plugin %s` to install it",
            os.Args[0])
        os.Exit(1)
    }

    cfg.Initialize()

    plugins.Execute(new(IdbPlugin))
}
