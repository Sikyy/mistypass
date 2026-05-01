package httpx

import (
	"net/http"
	"time"
)

type networkTopologyNode struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Label    string `json:"label"`
	Status   string `json:"status"`
	IP       string `json:"ip,omitempty"`
	Protocol string `json:"protocol,omitempty"`
	LastSeen string `json:"last_seen,omitempty"`
}

type networkTopologyEdge struct {
	Source   string `json:"source"`
	Target   string `json:"target"`
	Protocol string `json:"protocol"`
}

type networkTopologySummary struct {
	TotalGateways int `json:"total_gateways"`
	TotalDevices  int `json:"total_devices"`
	Online        int `json:"online"`
	Offline       int `json:"offline"`
}

type networkTopologyResponse struct {
	Nodes   []networkTopologyNode  `json:"nodes"`
	Edges   []networkTopologyEdge  `json:"edges"`
	Summary networkTopologySummary `json:"summary"`
}

func (s *server) getNetworkTopology(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := s.resolveTenantID(w, r, r.URL.Query().Get("tenant_id"))
	if !ok {
		return
	}

	gateways := s.gatewaySvc.List(tenantID)

	// Cloud node is always present and always online.
	nodes := []networkTopologyNode{
		{ID: "cloud", Type: "cloud", Label: "MistyPass Cloud", Status: "online"},
	}
	var edges []networkTopologyEdge

	totalGateways := len(gateways)
	totalDevices := 0
	online := 1 // cloud node
	offline := 0

	for _, gw := range gateways {
		gwNodeID := "gw_" + gw.ID

		gwStatus := gatewayOnlineStatus(gw.Status, gw.LastSeenAt)
		nodes = append(nodes, networkTopologyNode{
			ID:       gwNodeID,
			Type:     "gateway",
			Label:    "Gateway " + gw.SerialNumber,
			Status:   gwStatus,
			LastSeen: gw.LastSeenAt.Format(time.RFC3339),
		})
		edges = append(edges, networkTopologyEdge{
			Source:   "cloud",
			Target:   gwNodeID,
			Protocol: "https+nats",
		})
		if gwStatus == "online" {
			online++
		} else {
			offline++
		}

		for _, dev := range gw.Devices {
			totalDevices++
			devNodeID := "dev_" + dev.ID

			devType := deviceKindToNodeType(dev.Kind)
			devStatus := gatewayOnlineStatus(dev.Status, dev.LastSeenAt)

			nodes = append(nodes, networkTopologyNode{
				ID:       devNodeID,
				Type:     devType,
				Label:    deviceLabel(dev.Kind, dev.SerialNumber),
				Status:   devStatus,
				Protocol: dev.Protocol,
				LastSeen: dev.LastSeenAt.Format(time.RFC3339),
			})
			edges = append(edges, networkTopologyEdge{
				Source:   gwNodeID,
				Target:   devNodeID,
				Protocol: deviceEdgeProtocol(dev.Protocol),
			})
			if devStatus == "online" {
				online++
			} else {
				offline++
			}
		}
	}

	writeJSON(w, http.StatusOK, networkTopologyResponse{
		Nodes: nodes,
		Edges: edges,
		Summary: networkTopologySummary{
			TotalGateways: totalGateways,
			TotalDevices:  totalDevices,
			Online:        online,
			Offline:       offline,
		},
	})
}

// gatewayOnlineStatus returns "online" when the stored status is "online" and
// the device was seen within the last 5 minutes; otherwise "offline".
func gatewayOnlineStatus(status string, lastSeen time.Time) string {
	if status == "online" && time.Since(lastSeen) < 5*time.Minute {
		return "online"
	}
	return "offline"
}

func deviceKindToNodeType(kind string) string {
	switch kind {
	case "reader", "legacy_reader":
		return "reader"
	case "door_controller", "legacy_controller":
		return "controller"
	case "relay":
		return "relay"
	case "sensor":
		return "sensor"
	default:
		return kind
	}
}

func deviceLabel(kind, serial string) string {
	var prefix string
	switch kind {
	case "reader", "legacy_reader":
		prefix = "Reader"
	case "door_controller", "legacy_controller":
		prefix = "Controller"
	case "relay":
		prefix = "Relay"
	case "sensor":
		prefix = "Sensor"
	default:
		prefix = "Device"
	}
	return prefix + " " + serial
}

func deviceEdgeProtocol(protocol string) string {
	switch protocol {
	case "rs485":
		return "rs485"
	case "osdp_v2":
		return "osdp_v2"
	case "ble":
		return "ble"
	case "wiegand_26", "wiegand_34":
		return protocol
	default:
		return "usb"
	}
}
