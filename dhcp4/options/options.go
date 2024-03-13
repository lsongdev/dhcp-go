package options

// optionCode is a DHCP option code.
type OptionCode uint8

type Option interface {
	Code() OptionCode
	Encode() []byte
	Decode([]byte) Option
	String() string
}

const (
	OptionCodeSubnetMask                      OptionCode = 1
	OptionCodeTimeOffset                      OptionCode = 2
	OptionCodeRouter                          OptionCode = 3
	OptionCodeTimeServer                      OptionCode = 4
	OptionCodeNameServer                      OptionCode = 5
	OptionCodeDomainNameServer                OptionCode = 6
	OptionCodeLogServer                       OptionCode = 7
	OptionCodeCookieServer                    OptionCode = 8
	OptionCodeLPRServer                       OptionCode = 9
	OptionCodeImpressServer                   OptionCode = 10
	OptionCodeResourceLocationServer          OptionCode = 11
	OptionCodeHostName                        OptionCode = 12
	OptionCodeBootFileSize                    OptionCode = 13
	OptionCodeDumpFile                        OptionCode = 14
	OptionCodeDomainName                      OptionCode = 15
	OptionCodeSwapServer                      OptionCode = 16
	OptionCodeRootPath                        OptionCode = 17
	OptionCodeExtensionsPath                  OptionCode = 18
	OptionCodeIPForwarding                    OptionCode = 19
	OptionCodeNonLocalSourceRouting           OptionCode = 20
	OptionCodePolicyFilter                    OptionCode = 21
	OptionCodeMaxDatagramReassembly           OptionCode = 22
	OptionCodeDefaultIPTimeToLive             OptionCode = 23
	OptionCodePathMTUAgingTimeout             OptionCode = 24
	OptionCodePathMTUPlateauTable             OptionCode = 25
	OptionCodeInterfaceMTU                    OptionCode = 26
	OptionCodeAllSubnetsLocal                 OptionCode = 27
	OptionCodeBroadcastAddress                OptionCode = 28
	OptionCodePerformMaskDiscovery            OptionCode = 29
	OptionCodeMaskSupplier                    OptionCode = 30
	OptionCodePerformRouterDiscovery          OptionCode = 31
	OptionCodeRouterSolicitationAddress       OptionCode = 32
	OptionCodeStaticRoute                     OptionCode = 33
	OptionCodeTrailerEncapsulation            OptionCode = 34
	OptionCodeARPCacheTimeout                 OptionCode = 35
	OptionCodeEthernetEncapsulation           OptionCode = 36
	OptionCodeTcpDefaultTTL                   OptionCode = 37
	OptionCodeTcpKeepaliveInterval            OptionCode = 38
	OptionCodeTcpKeepaliveGarbage             OptionCode = 39
	OptionCodeNetworkInformationServiceDomain OptionCode = 40
	OptionCodeNetworkInformationServers       OptionCode = 41
	OptionCodeNetworkTimeProtocolServers      OptionCode = 42
	OptionCodeVendorSpecificInformation       OptionCode = 43
	OptionCodeNetBIOSNameServer               OptionCode = 44
	OptionCodeRequestedIPAddress              OptionCode = 50
	OptionCodeLeaseTime                       OptionCode = 51
	OptionCodeOverload                        OptionCode = 52
	OptionCodeMessageType                     OptionCode = 53
	OptionCodeServerIdentifier                OptionCode = 54
	OptionCodeParameterRequest                OptionCode = 55
	OptionCodeMessage                         OptionCode = 56
	OptionCodeMaximumMessageSize              OptionCode = 57
	OptionCodeRenewalTime                     OptionCode = 58
	OptionCodeRebindingTime                   OptionCode = 59
	OptionCodeVendorClassIdentifier           OptionCode = 60
	OptionCodeClientIdentifier                OptionCode = 61
	OptionCodeClientFullyQualifiedDomainName  OptionCode = 81
)

var optionTypes = map[OptionCode]Option{
	OptionCodeSubnetMask:         SubnetMaskOption{},
	OptionCodeRouter:             RouterOption{},
	OptionCodeDomainNameServer:   DomainNameServerOption{},
	OptionCodeHostName:           HostNameOption{},
	OptionCodeDomainName:         DomainNameOption{},
	OptionCodeBroadcastAddress:   BroadcastAddressOption{},
	OptionCodeRequestedIPAddress: RequestedIPAddressOption{},
	OptionCodeLeaseTime:          LeaseTimeOption{},
	OptionCodeMessageType:        MessageTypeOption{},
	OptionCodeServerIdentifier:   ServerIdentifierOption{},
	OptionCodeParameterRequest:   ParameterRequestOption{},
	OptionCodeMessage:            MessageOption{},
	OptionCodeMaximumMessageSize: MaximumMessageSizeOption{},
	OptionCodeRenewalTime:        RenewalTimeOption{},
	OptionCodeRebindingTime:      RebindingTimeOption{},
	OptionCodeClientIdentifier:   ClientIdentifierOption{},
	108:                          Option108{},
	138:                          Option138{},
	// option95: LDAP
	// option108: IPv6-Only Preferred
	// option114: DHCP Captive-Portal(URL)
	// option118: Subnet Selection Option
	// option119: DNS Domain Search List
	// option121: Classless Static Route
	// option252: Private/Proxy autodiscovery
}

func ParseOption(code OptionCode, data []byte) Option {
	option, ok := optionTypes[code]
	if !ok {
		option = NewRawOption(code)
	}
	return option.Decode(data)
}
