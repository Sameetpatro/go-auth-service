package websocket

// Broadcaster adapts Hub to service-layer interfaces.
type Broadcaster struct {
	hub *Hub
}

func NewBroadcaster(hub *Hub) *Broadcaster {
	return &Broadcaster{hub: hub}
}

func (b *Broadcaster) BroadcastGuestCheckedIn(data interface{}) {
	b.hub.Broadcast(EventGuestCheckedIn, data)
}

func (b *Broadcaster) BroadcastGuestAdded(data interface{}) {
	b.hub.Broadcast(EventNewGuestAdded, data)
}

func (b *Broadcaster) BroadcastCoordinatorCreated(data interface{}) {
	b.hub.Broadcast(EventCoordinatorCreated, data)
}

func (b *Broadcaster) BroadcastDashboardUpdated() {
	b.hub.Broadcast(EventDashboardUpdated, map[string]string{"status": "updated"})
}

func (b *Broadcaster) BroadcastInsightsUpdated() {
	b.hub.Broadcast(EventInsightsUpdated, map[string]string{"status": "updated"})
}
