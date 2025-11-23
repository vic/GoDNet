package deltanet

const MaxPriority = 64

type Scheduler struct {
	queues [MaxPriority]chan *Wire
	signal chan struct{}
}

func NewScheduler() *Scheduler {
	s := &Scheduler{
		signal: make(chan struct{}, 10000),
	}
	for i := range s.queues {
		s.queues[i] = make(chan *Wire, 1024)
	}
	return s
}

func (s *Scheduler) Push(w *Wire, depth int) {
	if depth < 0 {
		depth = 0
	}
	if depth >= MaxPriority {
		depth = MaxPriority - 1
	}
	s.queues[depth] <- w
	select {
	case s.signal <- struct{}{}:
	default:
		// Signal buffer full, workers should be busy enough
	}
}

func (s *Scheduler) Pop() *Wire {
	for {
		// Scan for highest priority (lowest depth index)
		for i := 0; i < MaxPriority; i++ {
			select {
			case w := <-s.queues[i]:
				return w
			default:
				continue
			}
		}
		// No work found, wait for signal
		<-s.signal
	}
}
