package ship

const CmiTypeData = 2

type CmiDataMsg struct {
	Data `json:"data"`
}

type Data struct {
	Header    `json:"header"`
	Payload   interface{} `json:"payload"`
	Extension []Extension `json:"extension"`
}

type Header struct {
	ProtocolID string `json:"protocolId"`
}

type Extension struct {
	ExtensionID string `json:"extensionId"`
	Binary      []byte `json:"binary,omitempty"`
	String      string `json:"string,omitempty"`
}
