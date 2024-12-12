package handlers

import (
	"encoding/json"
	"log"

	"github.com/MikeDev101/cloudlink-phi/server/pkg/manager"
	"github.com/MikeDev101/cloudlink-phi/server/pkg/peer"
	"github.com/MikeDev101/cloudlink-phi/server/pkg/signaling/message"
	"github.com/MikeDev101/cloudlink-phi/server/pkg/structs"
)

// Handles the ICE opcode. This function takes a client,
// an SDP ICE candidate, and forwards the candidate to the desired peer. If
// the peer does not exist or isn't in the same lobby, the function sends the client a
// PEER_INVALID packet.
func ICE(s *structs.Server, client *structs.Client, packet *structs.SignalPacket, rawpacket []byte) {

	// Require the peer to be in a lobby
	if !client.AmIInALobby() {
		err := message.Code(
			client,
			"CONFIG_REQUIRED",
			nil,
			packet.Listener,
			nil,
		)
		if err != nil {
			log.Printf("Send CONFIG_REQUIRED response to ICE opcode error: %s", err.Error())
		}
		return
	}

	// Read lobby settings. If the peer is the relay, handle the answer through the relay
	settings := manager.GetLobbySettings(s, client.Lobby, client.UGI)
	if packet.Recipient == "relay" {
		if !settings.UseServerRelay {
			return
		}

		// Read the raw packet as a relay packet
		reparsed := &structs.RelayInboundIcePacket{}
		if err := json.Unmarshal(rawpacket, &reparsed); err != nil {
			log.Printf("Unmarshal relay ICE packet error: %s", err.Error())
			return
		}

		relay := manager.GetRelay(s, client)

		// The candidate type cannot be a voice candidate since we're a server, not a person.
		if reparsed.Payload.Type == structs.VOICE_CANDIDATE {
			log.Print("Handling ICE for relay peer can't be done: Got a voice candidate!")
			message.Code(
				client,
				"WARNING",
				"voice connections are not supported by the server relay",
				packet.Listener,
				nil,
			)
			return
		}

		peer.HandleIce(relay, reparsed.Payload.Contents)
		return
	}

	// Check if the desired peer exists. If it does, get the peer's connection
	peer := manager.GetByULID(s, packet.Recipient)
	if peer == nil {
		log.Printf("Failed to get ICE peer as it doesn't exist: %s", packet.Recipient)
		return
	}

	// If the peer is nil or not in the lobby, send a PEER_INVALID packet
	if !manager.IsClientInLobby(s, client.Lobby, client.UGI, peer) {
		err := message.Code(
			client,
			"PEER_INVALID",
			nil,
			packet.Listener,
			nil,
		)
		if err != nil {
			log.Printf("Send PEER_INVALID response to ICE opcode error: %s", err.Error())
		}
		return
	}

	// Relay the ICE candidate to the desired peer
	err := message.Code(
		peer,
		"ICE",
		packet.Payload,
		"",
		&structs.PeerInfo{
			ID:   client.ID,
			User: client.Username,
		},
	)
	if err != nil {
		log.Printf("Relay ICE opcode error: %s", err.Error())
	}

	// Tell the original client that the ICE candidate was relayed
	err = message.Code(
		client,
		"RELAY_OK",
		nil,
		packet.Listener,
		nil,
	)
	if err != nil {
		log.Printf("Send RELAY_OK response to ICE opcode error: %s", err.Error())
	}
}