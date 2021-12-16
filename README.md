- [hpegl-provider-lib](#hpegl-provider-lib)
    * [Introduction](#introduction)
    * [pkg/client](#pkgclient)
        + [Use in service provider repos](#use-in-service-provider-repos)
            - [GetClientFromMetaMap function](#getclientfrommetamap-function)
        + [Use in hpegl provider](#use-in-hpegl-provider)
    * [pkg/gltform](#pkggltform)
        + [Use in service provider repos](#use-in-service-provider-repos-1)
        + [Use in hpegl provider](#use-in-hpegl-provider-1)
    * [pkg/provider](#pkgprovider)
        + [Use in service provider repos](#use-in-service-provider-repos-2)
        + [Use in hpegl provider](#use-in-hpegl-provider-2)
    * [pkg/registration](#pkgregistration)
        + [Use in service provider repos](#use-in-service-provider-repos-3)
            - [Resource and data-source naming](#resource-and-data-source-naming)
            - [Service block in the provider stanza](#service-block-in-the-provider-stanza)
        + [Use in hpegl provider](#use-in-hpegl-provider-3)
    * [pkg/token](#pkgtoken)
        + [Introduction](#introduction-1)
        + [pkg/token/common](#pkgtokencommon)
        + [pkg/token/retrieve](#pkgtokenretrieve)
            - [Use in service provider repos](#use-in-service-provider-repos-4)
            - [Use in hpegl provider](#use-in-hpegl-provider-4)
        + [pkg/token/serviceclient](#pkgtokenserviceclient)
            - [Use in service provider repos](#use-in-service-provider-repos-5)
            - [Use in hpegl provider](#use-in-hpegl-provider-5)
	* [pkg/utils](#pkgutils)
		+ [Example use](#example-use)

# hpegl-provider-lib

## Introduction

This repo contains library code that is used by terraform-provider-hpegl and the service provider repos.

## pkg/client

This package defines an interface that must be satisfied by all service provider client creation code.  The interface
is:

```go
package client

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// Initialisation interface, service Client creation code will have to satisfy this interface
// The hpegl provider will iterate over a slice of these to initialise service clients
type Initialisation interface {
	// NewClient is run by hpegl to initialise the service client
	NewClient(r *schema.ResourceData) (interface{}, error)

	// ServiceName is used by hpegl, it returns the key to be used for the client returned by NewClient
	// in the map[string]interface{} passed-down to provider code by terraform
	ServiceName() string
}
```
The *schema.ResourceData is the provider config stanza.

### Use in service provider repos

Service provider repos will define an exported InitialiseClient{} struct that implements this interface.
An example of this:
```go
package client

import (
	"fmt"
	"os"

	api_client "github.com/HewlettPackard/hpegl-vmaas-cmp-go-sdk/pkg/client"
	"github.com/HewlettPackard/hpegl-vmaas-cmp-go-sdk/pkg/models"
	cmp_client "github.com/HewlettPackard/hpegl-vmaas-terraform-resources/internal/cmp"
	"github.com/HewlettPackard/hpegl-vmaas-terraform-resources/pkg/constants"
	"github.com/HewlettPackard/hpegl-vmaas-terraform-resources/pkg/utils"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hewlettpackard/hpegl-provider-lib/pkg/client"
	"github.com/tshihad/tftags"
)

// keyForGLClientMap is the key in the map[string]interface{} that is passed down by hpegl used to store *Client
// This must be unique, hpegl will error-out if it isn't
const keyForGLClientMap = "vmaasClient"

// Assert that InitialiseClient satisfies the client.Initialisation interface
var _ client.Initialisation = (*InitialiseClient)(nil)

// Client is the client struct that is used by the provider code
type Client struct {
	CmpClient *cmp_client.Client
}

// InitialiseClient is imported by hpegl from each service repo
type InitialiseClient struct{}

// NewClient takes an argument of all of the provider.ConfigData, and returns an interface{} and error
// If there is no error interface{} will contain *Client.
// The hpegl provider will put *Client at the value of keyForGLClientMap (returned by ServiceName) in
// the map of clients that it creates and passes down to provider code.  hpegl executes NewClient for each service.
func (i InitialiseClient) NewClient(r *schema.ResourceData) (interface{}, error) {
	var tfprovider models.TFProvider
	if err := tftags.Get(r, &tfprovider); err != nil {
		return nil, err
	}
	// Create VMaas Client
	client := new(Client)

	cfg := api_client.Configuration{
		Host:          serviceURL,
		DefaultHeader: getHeaders(),
		DefaultQueryParams: map[string]string{
			constants.SpaceKey:    tfprovider.Vmaas.SpaceName,
			constants.LocationKey: tfprovider.Vmaas.Location,
		},
	}
	apiClient := api_client.NewAPIClient(&cfg)
	utils.SetMeta(apiClient, r)
	client.CmpClient = cmp_client.NewClient(apiClient, cfg)

	return client, nil
}

// ServiceName is used to return the value of keyForGLClientMap, for use by hpegl
func (i InitialiseClient) ServiceName() string {
	return keyForGLClientMap
}
// GetClientFromMetaMap is a convenience function used by provider code to extract *Client from the
// meta argument passed-in by terraform
func GetClientFromMetaMap(meta interface{}) (*Client, error) {
	cli := meta.(map[string]interface{})[keyForGLClientMap]
	if cli == nil {
		return nil, fmt.Errorf("client is not initialised, make sure that vmaas block is defined in hpegl stanza")
	}

	return cli.(*Client), nil
}

// Get env configurations for VmaaS services
func getHeaders() map[string]string {
	token := os.Getenv("HPEGL_IAM_TOKEN")
	header := make(map[string]string)
	serviceURL = utils.GetServiceEndpoint()
	if utils.GetEnvBool(constants.MockIAMKey) {
		header["subject"] = os.Getenv(constants.CmpSubjectKey)
		header["Authorization"] = token
	}

	return header
}

```

Note the following:
* We define an IntialiseClient{} struct that implements the client.Initialisation interface
* We have a Client{} struct that contains:
    * An instance of a service client
    * An IAMToken for use with the client <br>
  We expect that all services will have a Client{} struct similar to this
* We store a unique key for the service client as a constant keyForGLClientMap, this key must be unique
    for each service.  hpegl will check that the service client keys are unique on start-up, and will
    error out if it detects a repeated key.
* The unique key is returned by ServiceName()
* We've added a GetClientFromMetaMap() convenience function that is used by provider CRUD code to
    return *Client from the meta interface passed-in to the CRUD code by terraform, like so:
    ```go
	package resources

	import (
		"context"

		"github.com/HewlettPackard/hpegl-vmaas-terraform-resources/internal/utils"
		"github.com/HewlettPackard/hpegl-vmaas-terraform-resources/pkg/client"
		"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
		"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	)

	func DatastoreData() *schema.Resource {
		return &schema.Resource{
			Schema: map[string]*schema.Schema{
				"name": {
					Type:        schema.TypeString,
					Required:    true,
					Description: f(generalNamedesc, "datastore", "datastore"),
				},
				"cloud_id": {
					Type:        schema.TypeString,
					Required:    true,
					Description: f(generalDDesc, "cloud"),
				},
			},
			ReadContext: datastoreReadContext,
			Description: `The ` + DSDatastore + ` data source can be used to discover the ID of a hpegl vmaas datastore.
			This can then be used with resources or data sources that require a ` + DSDatastore + `,
			such as the ` + ResInstance + ` resource.`,
			SchemaVersion:  0,
			StateUpgraders: nil,
			Importer: &schema.ResourceImporter{
				StateContext: schema.ImportStatePassthroughContext,
			},
		}
	}

	func datastoreReadContext(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
		c, err := client.GetClientFromMetaMap(meta)
		if err != nil {
			return diag.FromErr(err)
		}

		data := utils.NewData(d)
		err = c.CmpClient.Datastore.Read(ctx, data, meta)
		if err != nil {
			return diag.FromErr(err)
		}

		return nil
	}
    ```

Note that we've added a GetToken() convenience function that is used by provider CRUD code to fetch the
[Token Retrieve Function](#pkgtokenretrieve) - see [here](#use-in-service-provider-repos-4)

#### GetClientFromMetaMap function

Note that GetClientFromMetaMap can return an error.  This is because the example service shown here uses a
[service block in the provider stanza](#service-block-in-the-provider-stanza).
These blocks are optional, since a customer may not want to use
the related service in a terraform run.  The blocks are intended for use with client initialisation.
If a service needs a provider block for client initialisation and one isn't present then we expect the NewClient()
function to return nil.  The GetClientFromMetaMap() function will return an error if the meta-map entry for
the service is nil.  By raising a diag.FromErr with this error Terraform will display the error message to
the user on the console, who can take action (i.e. add a service block to the provider stanza).

### Use in hpegl provider

In the hpegl provider a slice of service implementations of this interface is created and iterated over to
populate the map[string]interface{} that is provided as the meta argument to service provider code by
hpegl.  This slice is iterated over as follows:

```go
package client

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"

	"github.com/hewlettpackard/hpegl-provider-lib/pkg/provider"

	"github.com/hpe-hcss/terraform-provider-hpegl/internal/services/clients"
)

func NewClientMap(config provider.ConfigData) (map[string]interface{}, diag.Diagnostics) {
	c := make(map[string]interface{})

	// Iterate over services
	for _, cli := range clients.InitialiseClients() {
		scli, err := cli.NewClient(config)
		if err != nil {
			return nil, diag.Errorf("error in creating client %s: %s", cli.ServiceName(), err)
		}

		// Check that cli.ServiceName() value is unique
		if _, ok := c[cli.ServiceName()]; ok {
			return nil, diag.Errorf("%s client key is not unique", cli.ServiceName())
		}

		// Add service client to map
		c[cli.ServiceName()] = scli
	}
	
	return c, nil
}

```

## pkg/provider

This defines a number of functions used in creating the plugin.ProviderFunc object that is used to
expose the provider code to Terraform.  This package is used by hpegl to expose the hpegl provider that spans
all supported services, and is used by individual service provider repos to create a "dummy" provider restricted
to just the service covered by each repo.  This "dummy" provider can be used for development purposes and for
acceptance testing.  Note that the "dummy" provider takes the same set of config arguments as the hpegl provider,
but will only support CRUD operations on the relevant service objects.

### Use in service provider repos

This package is used as follows in a service provider repo:

```go
package testutils

import (
	"context"

	"github.com/HewlettPackard/hpegl-vmaas-terraform-resources/pkg/client"
	"github.com/HewlettPackard/hpegl-vmaas-terraform-resources/pkg/resources"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/plugin"
	"github.com/hewlettpackard/hpegl-provider-lib/pkg/provider"
	"github.com/hewlettpackard/hpegl-provider-lib/pkg/token/common"
	"github.com/hewlettpackard/hpegl-provider-lib/pkg/token/retrieve"
	"github.com/hewlettpackard/hpegl-provider-lib/pkg/token/serviceclient"
)

func ProviderFunc() plugin.ProviderFunc {
	return provider.NewProviderFunc(provider.ServiceRegistrationSlice(resources.Registration{}), providerConfigure)
}

func providerConfigure(p *schema.Provider) schema.ConfigureContextFunc { // nolint staticcheck
	return func(ctx context.Context, d *schema.ResourceData) (interface{}, diag.Diagnostics) {
		cli, err := client.InitialiseClient{}.NewClient(d)
		if err != nil {
			return nil, diag.Errorf("error in creating client: %s", err)
		}

		// Initialise token handler
		h, err := serviceclient.NewHandler(d)
		if err != nil {
			return nil, diag.FromErr(err)
		}

		// Returning a map[string]interface{} with the Client from pkg.client at the
		// key specified in that repo to ensure compatibility with the hpegl terraform
		// provider
		return map[string]interface{}{
			client.InitialiseClient{}.ServiceName(): cli,
			common.TokenRetrieveFunctionKey:         retrieve.NewTokenRetrieveFunc(h),
		}, nil
	}
}
```

Note the following:
* resources.Registration{} is the ServiceRegistration interface implementation for the service, and exposes the
    service resource CRUD operations along with any service block for inclusion in the provider stanza to the "dummy" provider
  
* providerConfigure returns a schema.ConfigureContextFunc which is used to configure the service client.  The
    client.InitialiseClient{} struct is the service implementation of the client Initialisation interface.
    We add code to initialise the IAM token Handler, use it to create a Token Retrieve Function and put it in
    a map[string]interface{} at the expected key. The client created by the InitialiseClient{}.NewClient()
    function is added to map[string]interface{} map at the key given by InitialiseClient{}.ServiceName().  This is
    to ensure compatibility with the hpegl provider.
  
ProviderFunc() above can then be used to create a "dummy" service-specific provider as follows:
```go
package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-sdk/v2/plugin"

	testutils "github.com/HewlettPackard/hpegl-vmaas-terraform-resources/internal/test-utils"
)

func main() {
	var debugMode bool

	flag.BoolVar(&debugMode, "debug", false, "set to true to run the provider with support for debuggers like delve")
	flag.Parse()
	opts := &plugin.ServeOpts{
		ProviderFunc: testutils.ProviderFunc(),
	}
	if debugMode {
		err := plugin.Debug(context.Background(), "terraform.example.com/vmaas/hpegl", opts)
		if err != nil {
			log.Fatal(err.Error())
		}

		return
	}

	plugin.Serve(opts)
}
```

ProviderFunc() can also be used in acceptance tests.

### Use in hpegl provider

The use of these functions in the hpegl provider is very similar to that in the service provider repos.
The differences are:
* A slice of registration.ServiceRegistration{} implementations one from each service is passed in to
    NewProviderFunc
* A map[string]interface{} that contains all of the supported service clients is returned by providerConfigure

```go
package hpegl

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/plugin"

	"github.com/hewlettpackard/hpegl-provider-lib/pkg/provider"

	"github.com/hpe-hcss/terraform-provider-hpegl/internal/client"
	"github.com/hpe-hcss/terraform-provider-hpegl/internal/services/resources"
)

func ProviderFunc() plugin.ProviderFunc {
	return provider.NewProviderFunc(resources.SupportedServices(), providerConfigure)
}

func providerConfigure(p *schema.Provider) schema.ConfigureContextFunc { // nolint staticcheck
	return func(ctx context.Context, d *schema.ResourceData) (interface{}, diag.Diagnostics) {
		return client.NewClientMap(ctx, d)
	}
}
```

This ProviderFunc is used to create the hpegl Terraform provider:
```go
package main

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/plugin"

	"github.com/hpe-hcss/terraform-provider-hpegl/internal/hpegl"
)

func main() {
	plugin.Serve(&plugin.ServeOpts{
		ProviderFunc: hpegl.ProviderFunc(),
	})
}
```

## pkg/registration

This package defines an interface that must be defined by all service repos to associate resource and data-source
code with hcl config names in addition to specifying a service block for inclusion in the provider stanza:

```go
package registration

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

type ServiceRegistration interface {
	// Name is the name of this Service - a mnemonic.  The value will be used to
	// set the name used for this service's entry in the provider schema
	Name() string

	// SupportedDataSources returns the supported Data Sources supported by this Service
	SupportedDataSources() map[string]*schema.Resource

	// SupportedResources returns the supported Resources supported by this Service
	SupportedResources() map[string]*schema.Resource

	// ProviderSchemaEntry returns the provider-level resource schema entry for this service
	// We will convert this into a schema.Schema of TypeSet in the provider
	// These blocks are marked as optional, it is up to the service-provider code to check that
	// the relevant service block is present if it is needed.
	ProviderSchemaEntry() *schema.Resource
}
```

### Use in service provider repos

Service provider repos will define an exported Registration{} struct that implements this interface.
An example of this:
```go
package resources

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	"github.com/HewlettPackard/hpegl-vmaas-terraform-resources/internal/resources"
	"github.com/HewlettPackard/hpegl-vmaas-terraform-resources/pkg/constants"
	"github.com/hewlettpackard/hpegl-provider-lib/pkg/registration"
)

// Assert that Registration implements the ServiceRegistration interface
var _ registration.ServiceRegistration = (*Registration)(nil)

type Registration struct{}

func (r Registration) Name() string {
	return constants.ServiceName
}

func (r Registration) SupportedDataSources() map[string]*schema.Resource {
	return map[string]*schema.Resource{
		resources.DSNetwork:          resources.NetworkData(),
		resources.DSNetworkType:      resources.NetworkTypeData(),
		resources.DSNetworkPool:      resources.NetworkPoolData(),
		resources.DSLayout:           resources.LayoutData(),
		resources.DSGroup:            resources.GroupData(),
		resources.DSPlan:             resources.PlanData(),
		resources.DSCloud:            resources.CloudData(),
		resources.DSResourcePool:     resources.ResourcePoolData(),
		resources.DSDatastore:        resources.DatastoreData(),
		resources.DSPowerSchedule:    resources.PowerScheduleData(),
		resources.DSTemplate:         resources.TemplateData(),
		resources.DSEnvironment:      resources.EnvironmentData(),
		resources.DSNetworkInterface: resources.NetworkInterfaceData(),
		resources.DSCloudFolder:      resources.CloudFolderData(),
		resources.DSRouter:           resources.RouterData(),
		resources.DSNetworkDomain:    resources.DomainData(),
		resources.DSNetworkProxy:     resources.NetworkProxyData(),
	}
}

func (r Registration) SupportedResources() map[string]*schema.Resource {
	return map[string]*schema.Resource{
		resources.ResInstance:                resources.Instances(),
		resources.ResInstanceClone:           resources.InstancesClone(),
		resources.ResNetwork:                 resources.Network(),
		resources.ResRouter:                  resources.Router(),
		resources.ResRouterNat:               resources.RouterNatRule(),
		resources.ResRouterFirewallRuleGroup: resources.RouterFirewallRuleGroup(),
		resources.ResRouterRoute:             resources.RouterRoute(),
		resources.ResRouterBgpNeighbor:       resources.RouterBgpNeighbor(),
	}
}

func (r Registration) ProviderSchemaEntry() *schema.Resource {
	return &schema.Resource{
		Schema: map[string]*schema.Schema{
			constants.LOCATION: {
				Type:        schema.TypeString,
				Required:    true,
				DefaultFunc: schema.EnvDefaultFunc("HPEGL_VMAAS_LOCATION", ""),
				Description: "Location of GL VMaaS Service, can also be set with the HPEGL_VMAAS_LOCATION env var.",
			},
			constants.SPACENAME: {
				Type:        schema.TypeString,
				Required:    true,
				DefaultFunc: schema.EnvDefaultFunc("HPEGL_VMAAS_SPACE_NAME", ""),
				Description: "IAM Space name of the GL VMaaS Service, can also be set with the HPEGL_VMAAS_SPACE_NAME env var.",
			},
		},
	}
}
```

#### Resource and data-source naming

Note the following:
* The keys returned by SupportedDataSources and SupportedResources correspond to resource and data-source definitions in hcl
* We are imposing the following key naming format:
    ```bash
    hpegl_<service mnemonic>_<service resource or data-source name>
    ```

#### Service block in the provider stanza

The *schema.Resource returned by ProviderSchemaEntry is added to the map[string]*schema.Schema{} map as a
TypeSet with a maximum size of 1 with the key provided by the Name() function.  Using a TypeSet in this way
- i.e. with a maximum size of 1 - seems to be the canonical way of adding configuration blocks to
terraform.  Note the following:
  
* The intention is that this block will be used for client initialisation in NewClient()
* The block is marked as optional, since we do not want to force users to have to define blocks for
    GreenLake services that they are not using in a terraform run
* If a service team doesn't want a block in the provider stanza, then ProviderSchemaEntry() should return
    nil
* We have added a helper function to pkg/client - GetServiceSettingsMap(key string, r *schema.ResourceData)
    - which can be used by NewClient() code to fetch the service block entries.  This function will
    return an error if there is no service block.  See [earlier](#getclientfrommetamap-function) for
    the implications of using a service block.


## pkg/token

### Introduction
This directory contains code to create and refresh tokens.  Every token creation method will define a handler
specific to it.  The way that this works is as follows:

* Each handler creates two channels:
    * a "resultCh" which is used to communicate the token along with an error in a Result struct to a function
      used by provider code which listens on the channel
    * an "exitCh" which is used by provider code to signal to the handler thread which presents tokens on "resultCh"
      to exit
* Each handler creates a thread (a go routine) which presents the Result struct obtained by running a "retrieve" function
    on "resultCh".  The thread also listens for a signal on "exitCh" to exit
* The handler "retrieve" function stashes a token in the handler.  If the token is about to expire in common.TimeToTokenExpiry
    seconds or less then a new token is obtained from IAM.
* The handler implements a simple interface common.TokenChannelInterface which returns the "resultCh" and the "exitCh"
* The hpegl provider instantiates the appropriate handler, and passes it down to retrieve.NewTokenRetrieveFunc which expects
    to get a common.TokenChannelInterface.  The interface is executed to get the "resultCh" and the "exitCh".
    A function of type TokenRetrieveFuncCtx is returned which takes a context and uses the channels passed-in to either return
    a token (and error) or signal the handler retrieve function to exit by writing into "exitCh" if the context is cancelled.
* The TokenRetrieveFuncCtx created is stashed in the map[string]interface{} passed down to the provider code at the
    common.TokenRetrieveFunctionKey key for execution by the provider code.
  
### pkg/token/common

Constants, a struct and an interface that are used by the retrieve package and by all token Handlers:
```go
package common

const (
	TokenRetrieveFunctionKey = "tokenRetrieveFunc"
	// TimeToTokenExpiry is seconds in int64, not time.Second
	// This constant should be used in all handler code
	TimeToTokenExpiry = 60
)

// Result the result struct sent back on the resultCh of a token Handler
type Result struct {
	Token string
	Err   error
}

// TokenChannelInterface the interface that is implemented by a token Handler
// This interface is used in retrieve.NewTokenRetrieveFunc
type TokenChannelInterface interface {
	TokenChannels() (chan Result, chan int)
}
```

### pkg/token/retrieve

The retrieve package used to construct a retrieve.TokenRetrieveFuncCtx function for use in terraform provider
code.

#### Use in service provider repos

In service provider code we assert that the object stored in the map[string]interface{} passed down from Terraform
at the key common.TokenRetrieveFunctionKey is of type retrieve.TokenRetrieveFuncCtx and execute it.  This
is wrapped-up in a convenience function GetToken as follows:

```go
// GetToken is a convenience function used by provider code to extract retrieve.TokenRetrieveFuncCtx from
// the meta argument passed-in by terraform and execute it with the context ctx
func GetToken(ctx context.Context, meta interface{}) (string, error) {
    trf := meta.(map[string]interface{})[common.TokenRetrieveFunctionKey].(retrieve.TokenRetrieveFuncCtx)

    return trf(ctx)
}
```

This function is executed in CRUD code:
```go
func clusterBlueprintCreateContext(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	_, err := client.GetClientFromMetaMap(meta)
	if err != nil {
		return diag.FromErr(err)
	}
	_, err = client.GetToken(ctx, meta)
	if err != nil {
		return diag.Errorf("Error in getting token: %s", err)
	}

	return nil
}
```

#### Use in hpegl provider

In the hpegl provider we use retrieve.NewTokenRetrieveFunc with a token Handler to create the retrieve.TokenRetrieveFuncCtx
and put in the map[string]interface{} at key common.TokenRetrieveFunctionKey:

```go
func NewClientMap(ctx context.Context, d *schema.ResourceData) (map[string]interface{}, diag.Diagnostics) {
	c := make(map[string]interface{})

	// Initialise token handler
	h, err := serviceclient.NewHandler(d)
	if err != nil {
		return nil, diag.FromErr(err)
	}

	// Get token retrieve func
	trf := retrieve.NewTokenRetrieveFunc(h)
	c[common.TokenRetrieveFunctionKey] = trf

    ...
	
	return c, nil
}
```

### pkg/token/serviceclient

This is an implementation of a token Handler that uses service-client creds to get a token from IAM.

#### Use in service provider repos

In the service provider repos we use this Handler when creating a "dummy-provider", like so:
```go
package testutils

import (
	"context"

	"github.com/HewlettPackard/hpegl-vmaas-terraform-resources/pkg/client"
	"github.com/HewlettPackard/hpegl-vmaas-terraform-resources/pkg/resources"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/plugin"
	"github.com/hewlettpackard/hpegl-provider-lib/pkg/provider"
	"github.com/hewlettpackard/hpegl-provider-lib/pkg/token/common"
	"github.com/hewlettpackard/hpegl-provider-lib/pkg/token/retrieve"
	"github.com/hewlettpackard/hpegl-provider-lib/pkg/token/serviceclient"
)

func ProviderFunc() plugin.ProviderFunc {
	return provider.NewProviderFunc(provider.ServiceRegistrationSlice(resources.Registration{}), providerConfigure)
}

func providerConfigure(p *schema.Provider) schema.ConfigureContextFunc { // nolint staticcheck
	return func(ctx context.Context, d *schema.ResourceData) (interface{}, diag.Diagnostics) {
		cli, err := client.InitialiseClient{}.NewClient(d)
		if err != nil {
			return nil, diag.Errorf("error in creating client: %s", err)
		}

		// Initialise token handler
		h, err := serviceclient.NewHandler(d)
		if err != nil {
			return nil, diag.FromErr(err)
		}

		// Returning a map[string]interface{} with the Client from pkg.client at the
		// key specified in that repo to ensure compatibility with the hpegl terraform
		// provider
		return map[string]interface{}{
			client.InitialiseClient{}.ServiceName(): cli,
			common.TokenRetrieveFunctionKey:         retrieve.NewTokenRetrieveFunc(h),
		}, nil
	}
}
```

#### Use in hpegl provider

In the hpegl provider we initialise this Handler for use in creating retrieve.TokenRetrieveFuncCtx:

```go
func NewClientMap(ctx context.Context, d *schema.ResourceData) (map[string]interface{}, diag.Diagnostics) {
	c := make(map[string]interface{})

	// Initialise token handler
	h, err := serviceclient.NewHandler(d)
	if err != nil {
		return nil, diag.FromErr(err)
	}

	// Get token retrieve func
	trf := retrieve.NewTokenRetrieveFunc(h)
	c[common.TokenRetrieveFunctionKey] = trf

    ...
	
	return c, nil
}
```

## pkg/utils

This package provides utilities to read yaml config file values using the viper package. 
This package is intended to be used as part of the acceptance test framework for service teams and the hpegl-terraform-provider. 

The utils package has a ReadAccConfig() function. 
The function searches for a yaml config file with the name equal to the value of TF_ACC_CONFIG environment variable. 
TF_ACC environment variable must be set to true for the function to be executed. 
The code panics if the config file is not found.
```go
func ReadAccConfig(path string) {
	if os.Getenv("TF_ACC") == "true" {
		viper.AddConfigPath(path)
		viper.SetConfigType("yaml")
		viper.SetConfigName(os.Getenv("TF_ACC_CONFIG"))
		err := viper.ReadInConfig()
		if err != nil {
			panic("fatal error config file: " + err.Error())
		}
	}
}
```

### Example use
NOTE: Yaml config files must be stored in the root directory for all service teams. 

```go
package main

import (
	"github.com/hewlettpackard/hpegl-provider-lib/pkg/utils"
)

func main() {
	// Read config file for acceptance test if TF_ACC is set
	// path parameter equal to root directory
	utils.ReadAccConfig(".")
}
```