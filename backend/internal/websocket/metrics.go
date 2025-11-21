package websocket

import "github.com/prometheus/client_golang/prometheus"

var (
	wsConnections = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "chat_app_ws_connections",
			Help: "Current number of active websocket connections.",
		},
	)
	wsRooms = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "chat_app_ws_rooms",
			Help: "Current number of websocket rooms.",
		},
	)
	wsMessagesDelivered = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "chat_app_ws_messages_delivered_total",
			Help: "Total websocket messages delivered to clients.",
		},
	)
)

func init() {
	prometheus.MustRegister(wsConnections, wsRooms, wsMessagesDelivered)
}

func incConnections() {
	wsConnections.Inc()
}

func decConnections() {
	wsConnections.Dec()
}

func setRooms(count int) {
	wsRooms.Set(float64(count))
}

func addDelivered(count int) {
	wsMessagesDelivered.Add(float64(count))
}
