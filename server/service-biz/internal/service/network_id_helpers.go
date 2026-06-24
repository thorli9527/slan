package service

type dnsZoneIDProvider interface{ NewDNSZoneID() string }
type dnsRecordIDProvider interface{ NewDNSRecordID() string }
type publicMappingIDProvider interface{ NewPublicMappingID() string }
type securityGroupIDProvider interface{ NewSecurityGroupID() string }
type securityRuleIDProvider interface{ NewSecurityRuleID() string }
