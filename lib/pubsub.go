package lib

//Broker - Channels for the main Broker instance
type Broker struct {
	stopCh       chan struct{}
	publishCh    chan Event
	subCh        chan chan Event
	unsubCh      chan chan Event
	publishCmdCh chan CommandData
	subCmdCh     chan chan CommandData
	unsubCmdCh   chan chan CommandData
}

func NewBroker() *Broker {
	return &Broker{
		stopCh:       make(chan struct{}),
		publishCh:    make(chan Event, 1000),
		subCh:        make(chan chan Event, 1000),
		unsubCh:      make(chan chan Event, 1000),
		publishCmdCh: make(chan CommandData, 1000),
		subCmdCh:     make(chan chan CommandData, 1000),
		unsubCmdCh:   make(chan chan CommandData, 1000),
	}
}

func (b *Broker) Start() {
	subs := map[chan Event]struct{}{}
	for {
		select {
		case <-b.stopCh:
			return
		case msgCh := <-b.subCh:
			subs[msgCh] = struct{}{}
		case msgCh := <-b.unsubCh:
			delete(subs, msgCh)
		case msg := <-b.publishCh:
			for msgCh := range subs {
				// msgCh is buffered, use non-blocking send to protect the broker:
				select {
				case msgCh <- msg:
				default:
				}
			}
		}
	}
}
func (b *Broker) StartCommandChannel() {
	subs := map[chan CommandData]struct{}{}
	for {
		select {
		case <-b.stopCh:
			return
		case cmdCh := <-b.subCmdCh:
			subs[cmdCh] = struct{}{}
		case cmdCh := <-b.unsubCmdCh:
			delete(subs, cmdCh)
		case msg := <-b.publishCmdCh:
			for cmdCh := range subs {
				// cmdCh is buffered, use non-blocking send to protect the broker:
				select {
				case cmdCh <- msg:
				default:
				}
			}
		}
	}
}
func (b *Broker) CMDSubscribe() chan CommandData {
	cmdCh := make(chan CommandData, 1000)
	b.subCmdCh <- cmdCh
	return cmdCh
}
func (b *Broker) Stop() {
	close(b.stopCh)
}

func (b *Broker) Subscribe() chan Event {
	msgCh := make(chan Event, 1000)
	b.subCh <- msgCh
	return msgCh
}

func (b *Broker) Unsubscribe(msgCh chan Event) {
	b.unsubCh <- msgCh
}

func (b *Broker) Publish(msg Event) {
	b.publishCh <- msg
}

func (b *Broker) SubscribeCmdChannel() chan CommandData {
	cmdCh := make(chan CommandData, 1000)
	b.subCmdCh <- cmdCh
	return cmdCh
}

func (b *Broker) UnsubscribeCmdChannel(cmd chan CommandData) {
	b.unsubCmdCh <- cmd
}

func (b *Broker) PublishCmdChannel(cmd CommandData) {
	b.publishCmdCh <- cmd
}
