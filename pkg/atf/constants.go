package atf

const providerStanza = `
	provider hpegl {
		vmaas {
		}
		caas {
		}
		metal {
		}
	}
`

var accTestPath = "../../acc-testcases"

const (
	accKey  = "acc"
	jsonKey = "json"
	tfKey   = "tf"

	randMaxLimit = 999999
)

