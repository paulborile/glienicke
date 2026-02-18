package nip11

import "encoding/json"

// RelayInformationDocument represents the NIP-11 relay information document.
type RelayInformationDocument struct {
	Name          string `json:"name,omitempty"`
	Description   string `json:"description,omitempty"`
	Pubkey        string `json:"pubkey,omitempty"`
	Contact       string `json:"contact,omitempty"`
	SupportedNIPs []int  `json:"supported_nips,omitempty"`
	Software      string `json:"software,omitempty"`
	Version       string `json:"version,omitempty"`
	Icon          string `json:"icon,omitempty"`
}

// ToJSON returns the JSON encoding of the document.
func (d *RelayInformationDocument) ToJSON() ([]byte, error) {
	return json.Marshal(d)
}
