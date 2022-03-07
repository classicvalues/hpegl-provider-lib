// (C) Copyright 2021 Hewlett Packard Enterprise Development LP

package provider

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/plugin"

	"github.com/hewlettpackard/hpegl-provider-lib/pkg/registration"
)

// ConfigureFunc is a type definition of a function that returns a ConfigureContextFunc object
// A function of this type is passed in to NewProviderFunc below
type ConfigureFunc func(p *schema.Provider) schema.ConfigureContextFunc

// NewProviderFunc is called from hpegl and service-repos to create a plugin.ProviderFunc which is used
// to define the provider that is exposed to Terraform.  The hpegl repo will use this to create a provider
// that spans all supported services.  A service repo will use this to create a "dummy" provider restricted
// to just the service that can be used for development purposes and for acceptance testing
func NewProviderFunc(reg []registration.ServiceRegistration, pf ConfigureFunc) plugin.ProviderFunc {
	return func() *schema.Provider {
		dataSources := make(map[string]*schema.Resource)
		resources := make(map[string]*schema.Resource)
		// providerSchema is the Schema for the provider
		providerSchema := Schema()
		for _, service := range reg {
			for k, v := range service.SupportedDataSources() {
				// We panic if the data-source name k is repeated in dataSources
				if _, ok := dataSources[k]; ok {
					panic(fmt.Sprintf("data-source name %s is repeated in service %s", k, service.Name()))
				}
				dataSources[k] = v
			}
			for k, v := range service.SupportedResources() {
				// We panic if the resource name k is repeated in resources
				if _, ok := resources[k]; ok {
					panic(fmt.Sprintf("resource name %s is repeated in service %s", k, service.Name()))
				}
				resources[k] = v
			}

			// TODO we can add a set of reserved providerSchema keys here to check against

			if service.ProviderSchemaEntry() != nil {
				// We panic if the service.Name() key is repeated in providerSchema
				if _, ok := providerSchema[service.Name()]; ok {
					panic(fmt.Sprintf("service name %s is repeated", service.Name()))
				}
				providerSchema[service.Name()] = convertToTypeSet(service.ProviderSchemaEntry())
			}
		}

		p := schema.Provider{
			Schema:         providerSchema,
			ResourcesMap:   resources,
			DataSourcesMap: dataSources,
			// Don't use the following field, experimental
			ProviderMetaSchema: nil,
			TerraformVersion:   "",
		}

		p.ConfigureContextFunc = pf(&p) // nolint staticcheck

		return &p
	}
}

func Schema() map[string]*schema.Schema {
	providerSchema := make(map[string]*schema.Schema)
	providerSchema["iam_token"] = &schema.Schema{
		Type:        schema.TypeString,
		Optional:    true,
		DefaultFunc: schema.EnvDefaultFunc("HPEGL_IAM_TOKEN", ""),
		Description: `The IAM token to be used with the client(s).  Note that in normal operation
                an API client is used.  Passing-in a token means that tokens will not be generated or refreshed.`,
	}

	providerSchema["iam_service_url"] = &schema.Schema{
		Type:        schema.TypeString,
		Optional:    true,
		DefaultFunc: schema.EnvDefaultFunc("HPEGL_IAM_SERVICE_URL", "https://client.greenlake.hpe.com/api/iam"),
		Description: `The IAM service URL to be used to generate tokens.  In the case of API-vended API clients
            (the default) then this should be set to the "issuer url" for the client.  In the case of non-API-vended
            API clients use the appropriate GL "client" URL. Can be set by HPEGL_IAM_SERVICE_URL env-var`,
	}

	providerSchema["api_vended_service_client"] = &schema.Schema{
		Type:        schema.TypeBool,
		Optional:    true,
		DefaultFunc: schema.EnvDefaultFunc("HPEGL_API_VENDED_SERVICE_CLIENT", true),
		Description: `Declare if the API client being used is an API-vended one or not.  Defaults to "true"
            i.e. the client is API-vended.  The value can be set using the HPEGL_API_VENDED_SERVICE_CLIENT env-var.`,
	}

	providerSchema["tenant_id"] = &schema.Schema{
		Type:        schema.TypeString,
		Optional:    true,
		DefaultFunc: schema.EnvDefaultFunc("HPEGL_TENANT_ID", ""),
		Description: "The tenant-id to be used, can be set by HPEGL_TENANT_ID env-var",
	}

	providerSchema["user_id"] = &schema.Schema{
		Type:        schema.TypeString,
		Optional:    true,
		DefaultFunc: schema.EnvDefaultFunc("HPEGL_USER_ID", ""),
		Description: "The user id to be used, can be set by HPEGL_USER_ID env-var",
	}

	providerSchema["user_secret"] = &schema.Schema{
		Type:        schema.TypeString,
		Optional:    true,
		DefaultFunc: schema.EnvDefaultFunc("HPEGL_USER_SECRET", ""),
		Description: "The user secret to be used, can be set by HPEGL_USER_SECRET env-var",
	}

	return providerSchema
}

// ServiceRegistrationSlice helper function to return []registration.ServiceRegistration from
// registration.ServiceRegistration input
// For use in service provider repos
func ServiceRegistrationSlice(reg registration.ServiceRegistration) []registration.ServiceRegistration {
	return []registration.ServiceRegistration{reg}
}

// convertToTypeSet helper function to take the *schema.Resource for a service and convert
// it into the element type of a TypeSet with exactly one element
func convertToTypeSet(r *schema.Resource) *schema.Schema {
	return &schema.Schema{
		Type:     schema.TypeSet,
		Optional: true,
		// Note that we only allow one of these sets, this is very important
		MaxItems: 1,
		// We put the *schema.Resource here
		Elem: r,
	}
}
