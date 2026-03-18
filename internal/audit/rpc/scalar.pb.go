// Code generated from ScalarDL 3.12 proto definitions. DO NOT EDIT.
// Source: scalar.proto (scalar-labs/scalardl)
// Generated types include only the message types required by the Tegata audit layer.
// Full proto source: https://github.com/scalar-labs/scalardl/blob/master/rpc/src/main/proto/scalar.proto

package rpc


// CertificateRegistrationRequest is sent to LedgerPrivileged.RegisterCert to
// associate an ECDSA certificate with an entity and key version.
// Field names follow the proto3 source: entity_id → EntityId, key_version → KeyVersion.
type CertificateRegistrationRequest struct {
	// EntityId is the ScalarDL entity identifier (e.g. "tegata-vault-alice").
	EntityId string `protobuf:"bytes,1,opt,name=entity_id,json=entityId,proto3" json:"entity_id,omitempty"`
	// KeyVersion is the monotonically increasing key version number.
	KeyVersion uint32 `protobuf:"varint,2,opt,name=key_version,json=keyVersion,proto3" json:"key_version,omitempty"`
	// CertPem is the PEM-encoded X.509 certificate.
	CertPem string `protobuf:"bytes,3,opt,name=cert_pem,json=certPem,proto3" json:"cert_pem,omitempty"`
}

func (x *CertificateRegistrationRequest) Reset()         {}
func (x *CertificateRegistrationRequest) String() string  { return x.EntityId }
func (x *CertificateRegistrationRequest) ProtoMessage()  {}

func (x *CertificateRegistrationRequest) GetEntityId() string {
	if x != nil {
		return x.EntityId
	}
	return ""
}

func (x *CertificateRegistrationRequest) GetKeyVersion() uint32 {
	if x != nil {
		return x.KeyVersion
	}
	return 0
}

func (x *CertificateRegistrationRequest) GetCertPem() string {
	if x != nil {
		return x.CertPem
	}
	return ""
}

// ContractExecutionRequest is sent to Ledger.ExecuteContract to run a
// registered HashStore contract (object.Put, object.Get, object.Validate).
type ContractExecutionRequest struct {
	// ContractId identifies the registered contract (e.g. "object.Put").
	ContractId string `protobuf:"bytes,1,opt,name=contract_id,json=contractId,proto3" json:"contract_id,omitempty"`
	// ContractArgument is a JSON string passed to the contract.
	ContractArgument string `protobuf:"bytes,2,opt,name=contract_argument,json=contractArgument,proto3" json:"contract_argument,omitempty"`
	// EntityId identifies the entity making the request.
	EntityId string `protobuf:"bytes,3,opt,name=entity_id,json=entityId,proto3" json:"entity_id,omitempty"`
	// KeyVersion is the key version corresponding to the signing certificate.
	KeyVersion uint32 `protobuf:"varint,4,opt,name=key_version,json=keyVersion,proto3" json:"key_version,omitempty"`
	// Signature is the ECDSA-SHA256 signature over the request fields.
	Signature []byte `protobuf:"bytes,6,opt,name=signature,proto3" json:"signature,omitempty"`
	// Nonce is a unique string (UUID v4) per request.
	Nonce string `protobuf:"bytes,10,opt,name=nonce,proto3" json:"nonce,omitempty"`
}

func (x *ContractExecutionRequest) Reset()         {}
func (x *ContractExecutionRequest) String() string  { return x.ContractId }
func (x *ContractExecutionRequest) ProtoMessage()  {}

func (x *ContractExecutionRequest) GetContractId() string {
	if x != nil {
		return x.ContractId
	}
	return ""
}

func (x *ContractExecutionRequest) GetContractArgument() string {
	if x != nil {
		return x.ContractArgument
	}
	return ""
}

func (x *ContractExecutionRequest) GetEntityId() string {
	if x != nil {
		return x.EntityId
	}
	return ""
}

func (x *ContractExecutionRequest) GetKeyVersion() uint32 {
	if x != nil {
		return x.KeyVersion
	}
	return 0
}

func (x *ContractExecutionRequest) GetSignature() []byte {
	if x != nil {
		return x.Signature
	}
	return nil
}

func (x *ContractExecutionRequest) GetNonce() string {
	if x != nil {
		return x.Nonce
	}
	return ""
}

// ContractExecutionResponse is returned by Ledger.ExecuteContract.
type ContractExecutionResponse struct {
	// ContractResult is a JSON string containing the contract execution result.
	ContractResult string `protobuf:"bytes,1,opt,name=contract_result,json=contractResult,proto3" json:"contract_result,omitempty"`
}

func (x *ContractExecutionResponse) Reset()         {}
func (x *ContractExecutionResponse) String() string  { return x.ContractResult }
func (x *ContractExecutionResponse) ProtoMessage()  {}

func (x *ContractExecutionResponse) GetContractResult() string {
	if x != nil {
		return x.ContractResult
	}
	return ""
}
