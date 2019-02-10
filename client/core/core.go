package core

import (
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"sliver/client/assets"
	consts "sliver/client/constants"
	pb "sliver/protobuf/client"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
)

const (
	randomIDSize = 16 // 64bits
)

var (
	// Events - Connect/Disconnect events
	Events = make(chan *pb.Event, 64)
)

// SliverServer - Server info
type SliverServer struct {
	Send      chan *pb.Envelope
	recv      chan *pb.Envelope
	responses *map[string]chan *pb.Envelope
	mutex     *sync.RWMutex
	Config    *assets.ClientConfig
}

// ResponseMapper - Maps recv'd envelopes to response channels
func (ss *SliverServer) ResponseMapper() {
	for envelope := range ss.recv {
		if envelope.ID != "" {
			ss.mutex.Lock()
			if resp, ok := (*ss.responses)[envelope.ID]; ok {
				resp <- envelope
			}
			ss.mutex.Unlock()
		} else if envelope.Type == consts.EventStr {
			event := &pb.Event{}
			proto.Unmarshal(envelope.Data, event)
			Events <- event
		}
	}
}

// RequestResponse - Send a request envelope and wait for a response (blocking)
func (ss *SliverServer) RequestResponse(envelope *pb.Envelope, timeout time.Duration) chan *pb.Envelope {
	reqID := RandomID()
	envelope.ID = reqID
	resp := make(chan *pb.Envelope)
	ss.AddRespListener(reqID, resp)
	ss.Send <- envelope
	respCh := make(chan *pb.Envelope)
	go func() {
		defer ss.RemoveRespListener(reqID)
		select {
		case respEnvelope := <-resp:
			respCh <- respEnvelope
		case <-time.After(timeout):
			respCh <- nil
		}
	}()
	return respCh
}

// AddRespListener - Add a response listener
func (ss *SliverServer) AddRespListener(requestID string, resp chan *pb.Envelope) {
	ss.mutex.Lock()
	defer ss.mutex.Unlock()
	(*ss.responses)[requestID] = resp
}

// RemoveRespListener - Remove a listener
func (ss *SliverServer) RemoveRespListener(requestID string) {
	ss.mutex.Lock()
	defer ss.mutex.Unlock()
	close((*ss.responses)[requestID])
	delete((*ss.responses), requestID)
}

// BindSliverServer - Bind send/recv channels to a server
func BindSliverServer(send chan *pb.Envelope, recv chan *pb.Envelope) *SliverServer {
	return &SliverServer{
		Send:      send,
		recv:      recv,
		responses: &map[string]chan *pb.Envelope{},
		mutex:     &sync.RWMutex{},
	}
}

// RandomID - Generate random ID of randomIDSize bytes
func RandomID() string {
	randBuf := make([]byte, 64) // 64 bytes of randomness
	rand.Read(randBuf)
	digest := sha256.Sum256(randBuf)
	return fmt.Sprintf("%x", digest[:randomIDSize])
}
