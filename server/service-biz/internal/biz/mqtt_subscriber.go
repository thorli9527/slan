package biz

import (
	"context"
	"log"
	"net"
	"time"
)

type mqttIncomingPublish struct {
	Topic    string
	Payload  []byte
	QoS      byte
	PacketID uint16
}

func (s *Server) startMQTTControlSubscriber() {
	if !s.mqtt.Enabled {
		return
	}
	go s.runMQTTControlSubscriber(context.Background())
}

func (s *Server) runMQTTControlSubscriber(ctx context.Context) {
	backoff := time.Second
	for {
		if err := s.runMQTTControlSubscriberOnce(ctx); err != nil {
			log.Printf("mqtt control subscriber disconnected: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		if backoff < 10*time.Second {
			backoff *= 2
		}
	}
}

func (s *Server) runMQTTControlSubscriberOnce(ctx context.Context) error {
	credential := serverMQTTCredential(s.mqtt, timeNow())
	if credential == nil {
		return nil
	}
	address, err := mqttBrokerAddress(s.mqtt.BrokerURL)
	if err != nil {
		return err
	}
	conn, err := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "tcp", address)
	if err != nil {
		return err
	}
	defer conn.Close()
	timeout := time.Duration(s.mqtt.PublishTimeoutMilliseconds) * time.Millisecond
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	refreshMQTTDeadline(conn, timeout)
	if _, err := conn.Write(mqttConnectPacket(credential.ClientID+"-subscriber", credential.Username, credential.Password)); err != nil {
		return err
	}
	if err := mqttReadConnAck(conn); err != nil {
		return err
	}
	topicFilter := mqttTopicRoot(s.mqtt) + "/devices/#"
	if _, err := conn.Write(mqttSubscribePacket(1, topicFilter, mqttQoSExactlyOnce)); err != nil {
		return err
	}
	if err := mqttReadSubAck(conn, 1); err != nil {
		return err
	}
	log.Printf("mqtt control subscriber connected topic=%s", topicFilter)
	_ = conn.SetDeadline(time.Time{})
	lastPing := time.Now()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		header, body, err := mqttReadPacket(conn)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				if time.Since(lastPing) >= 15*time.Second {
					_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
					if _, err := conn.Write([]byte{0xc0, 0x00}); err != nil {
						return err
					}
					lastPing = time.Now()
				}
				continue
			}
			return err
		}
		_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		switch header & 0xf0 {
		case 0x30:
			publish, err := mqttParsePublish(header, body)
			if err != nil {
				return err
			}
			if publish.QoS == 1 {
				_, _ = conn.Write([]byte{0x40, 0x02, byte(publish.PacketID >> 8), byte(publish.PacketID)})
			}
			if publish.QoS == 2 {
				_, _ = conn.Write([]byte{0x50, 0x02, byte(publish.PacketID >> 8), byte(publish.PacketID)})
			}
			if err := s.handleMQTTDevicePublish(ctx, publish.Topic, publish.Payload); err != nil {
				log.Printf("mqtt device publish rejected topic=%s: %v", publish.Topic, err)
			}
		case 0x60:
			if len(body) >= 2 {
				_, _ = conn.Write([]byte{0x70, 0x02, body[0], body[1]})
			}
		case 0xc0:
			_, _ = conn.Write([]byte{0xd0, 0x00})
		case 0xd0:
		default:
		}
	}
}
