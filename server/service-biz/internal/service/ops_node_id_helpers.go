package service

type operatorIDProvider interface{ NewOperatorID() string }
type relayNodeIDProvider interface{ NewRelayNodeID() string }
type punchNodeIDProvider interface{ NewPunchNodeID() string }
