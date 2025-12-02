package tcpconn

import (
	"errors"
	"fmt"
	"sync"
)

var (
	// ErrInvalidTransition возвращается при недопустимом переходе состояния
	ErrInvalidTransition = errors.New("invalid state transition")
	// ErrNilStateMachine возвращается при работе с nil машиной состояний
	ErrNilStateMachine = errors.New("state machine is nil")
)

// TCPState представляет состояние TCP соединения
type TCPState int

const (
	// CLOSED - соединение закрыто
	CLOSED TCPState = iota
	// LISTEN - сервер ожидает входящих соединений
	LISTEN
	// SYN_SENT - клиент отправил SYN и ждет SYN-ACK
	SYN_SENT
	// SYN_RECEIVED - сервер получил SYN и отправил SYN-ACK
	SYN_RECEIVED
	// ESTABLISHED - соединение установлено
	ESTABLISHED
	// FIN_WAIT_1 - активная сторона отправила FIN
	FIN_WAIT_1
	// FIN_WAIT_2 - активная сторона получила ACK на FIN
	FIN_WAIT_2
	// CLOSE_WAIT - пассивная сторона получила FIN
	CLOSE_WAIT
	// CLOSING - обе стороны одновременно закрывают соединение
	CLOSING
	// LAST_ACK - пассивная сторона отправила FIN и ждет ACK
	LAST_ACK
	// TIME_WAIT - ожидание перед окончательным закрытием
	TIME_WAIT
)

// String возвращает строковое представление состояния
func (s TCPState) String() string {
	switch s {
	case CLOSED:
		return "CLOSED"
	case LISTEN:
		return "LISTEN"
	case SYN_SENT:
		return "SYN_SENT"
	case SYN_RECEIVED:
		return "SYN_RECEIVED"
	case ESTABLISHED:
		return "ESTABLISHED"
	case FIN_WAIT_1:
		return "FIN_WAIT_1"
	case FIN_WAIT_2:
		return "FIN_WAIT_2"
	case CLOSE_WAIT:
		return "CLOSE_WAIT"
	case CLOSING:
		return "CLOSING"
	case LAST_ACK:
		return "LAST_ACK"
	case TIME_WAIT:
		return "TIME_WAIT"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", s)
	}
}

// TCPEvent представляет событие, влияющее на состояние TCP
type TCPEvent int

const (
	// PASSIVE_OPEN - пассивное открытие (сервер)
	PASSIVE_OPEN TCPEvent = iota
	// ACTIVE_OPEN - активное открытие (клиент)
	ACTIVE_OPEN
	// SYN - получен SYN пакет
	SYN
	// SYN_ACK - получен SYN-ACK пакет
	SYN_ACK
	// ACK - получен ACK пакет
	ACK
	// FIN - получен FIN пакет
	FIN
	// FIN_ACK - получен FIN-ACK пакет
	FIN_ACK
	// CLOSE - локальное закрытие соединения
	CLOSE
	// TIMEOUT - таймаут
	TIMEOUT
	// RST - сброс соединения
	RST
)

// String возвращает строковое представление события
func (e TCPEvent) String() string {
	switch e {
	case PASSIVE_OPEN:
		return "PASSIVE_OPEN"
	case ACTIVE_OPEN:
		return "ACTIVE_OPEN"
	case SYN:
		return "SYN"
	case SYN_ACK:
		return "SYN_ACK"
	case ACK:
		return "ACK"
	case FIN:
		return "FIN"
	case FIN_ACK:
		return "FIN_ACK"
	case CLOSE:
		return "CLOSE"
	case TIMEOUT:
		return "TIMEOUT"
	case RST:
		return "RST"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", e)
	}
}

// StateChangeCallback вызывается при изменении состояния
type StateChangeCallback func(oldState, newState TCPState, event TCPEvent)

// ErrorCallback вызывается при ошибке перехода
type ErrorCallback func(state TCPState, event TCPEvent, err error)

// TCPStateMachine представляет машину состояний TCP
type TCPStateMachine struct {
	currentState      TCPState
	mu                sync.RWMutex
	onStateChange     StateChangeCallback
	onError           ErrorCallback
	transitionHistory []StateTransition
	maxHistorySize    int
}

// StateTransition представляет запись о переходе состояния
type StateTransition struct {
	FromState TCPState
	ToState   TCPState
	Event     TCPEvent
}

// NewTCPStateMachine создает новую машину состояний TCP
func NewTCPStateMachine() *TCPStateMachine {
	return &TCPStateMachine{
		currentState:      CLOSED,
		transitionHistory: make([]StateTransition, 0),
		maxHistorySize:    100,
	}
}

// SetStateChangeCallback устанавливает callback для изменения состояния
func (sm *TCPStateMachine) SetStateChangeCallback(cb StateChangeCallback) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.onStateChange = cb
}

// SetErrorCallback устанавливает callback для ошибок
func (sm *TCPStateMachine) SetErrorCallback(cb ErrorCallback) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.onError = cb
}

// GetState возвращает текущее состояние
func (sm *TCPStateMachine) GetState() TCPState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.currentState
}

// ProcessEvent обрабатывает событие и изменяет состояние
func (sm *TCPStateMachine) ProcessEvent(event TCPEvent) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	oldState := sm.currentState
	newState, err := sm.transition(sm.currentState, event)

	if err != nil {
		if sm.onError != nil {
			sm.onError(sm.currentState, event, err)
		}
		return err
	}

	sm.currentState = newState

	// Сохраняем историю переходов
	sm.addToHistory(StateTransition{
		FromState: oldState,
		ToState:   newState,
		Event:     event,
	})

	if sm.onStateChange != nil {
		sm.onStateChange(oldState, newState, event)
	}

	return nil
}

// transition определяет переходы между состояниями
func (sm *TCPStateMachine) transition(state TCPState, event TCPEvent) (TCPState, error) {
	// RST всегда переводит в CLOSED
	if event == RST {
		return CLOSED, nil
	}

	switch state {
	case CLOSED:
		switch event {
		case PASSIVE_OPEN:
			return LISTEN, nil
		case ACTIVE_OPEN:
			return SYN_SENT, nil
		}

	case LISTEN:
		switch event {
		case SYN:
			return SYN_RECEIVED, nil
		case CLOSE:
			return CLOSED, nil
		}

	case SYN_SENT:
		switch event {
		case SYN_ACK:
			return ESTABLISHED, nil
		case SYN:
			return SYN_RECEIVED, nil
		case CLOSE:
			return CLOSED, nil
		case TIMEOUT:
			return CLOSED, nil
		}

	case SYN_RECEIVED:
		switch event {
		case ACK:
			return ESTABLISHED, nil
		case CLOSE:
			return FIN_WAIT_1, nil
		case TIMEOUT:
			return CLOSED, nil
		}

	case ESTABLISHED:
		switch event {
		case FIN:
			return CLOSE_WAIT, nil
		case CLOSE:
			return FIN_WAIT_1, nil
		}

	case FIN_WAIT_1:
		switch event {
		case FIN:
			return CLOSING, nil
		case ACK:
			return FIN_WAIT_2, nil
		case FIN_ACK:
			return TIME_WAIT, nil
		}

	case FIN_WAIT_2:
		switch event {
		case FIN:
			return TIME_WAIT, nil
		}

	case CLOSE_WAIT:
		switch event {
		case CLOSE:
			return LAST_ACK, nil
		}

	case CLOSING:
		switch event {
		case ACK:
			return TIME_WAIT, nil
		}

	case LAST_ACK:
		switch event {
		case ACK:
			return CLOSED, nil
		}

	case TIME_WAIT:
		switch event {
		case TIMEOUT:
			return CLOSED, nil
		}
	}

	return state, fmt.Errorf("%w: cannot transition from %s on event %s",
		ErrInvalidTransition, state, event)
}

// addToHistory добавляет переход в историю
func (sm *TCPStateMachine) addToHistory(transition StateTransition) {
	sm.transitionHistory = append(sm.transitionHistory, transition)

	// Ограничиваем размер истории
	if len(sm.transitionHistory) > sm.maxHistorySize {
		sm.transitionHistory = sm.transitionHistory[1:]
	}
}

// GetHistory возвращает историю переходов
func (sm *TCPStateMachine) GetHistory() []StateTransition {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	// Возвращаем копию истории
	history := make([]StateTransition, len(sm.transitionHistory))
	copy(history, sm.transitionHistory)
	return history
}

// ClearHistory очищает историю переходов
func (sm *TCPStateMachine) ClearHistory() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.transitionHistory = make([]StateTransition, 0)
}

// Reset сбрасывает машину состояний в начальное состояние
func (sm *TCPStateMachine) Reset() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.currentState = CLOSED
	sm.transitionHistory = make([]StateTransition, 0)
}

// IsConnected проверяет, установлено ли соединение
func (sm *TCPStateMachine) IsConnected() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.currentState == ESTABLISHED
}

// IsClosed проверяет, закрыто ли соединение
func (sm *TCPStateMachine) IsClosed() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.currentState == CLOSED
}

// IsClosing проверяет, находится ли соединение в процессе закрытия
func (sm *TCPStateMachine) IsClosing() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.currentState == FIN_WAIT_1 ||
		sm.currentState == FIN_WAIT_2 ||
		sm.currentState == CLOSING ||
		sm.currentState == LAST_ACK ||
		sm.currentState == TIME_WAIT ||
		sm.currentState == CLOSE_WAIT
}

// CanSendData проверяет, можно ли отправлять данные в текущем состоянии
func (sm *TCPStateMachine) CanSendData() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.currentState == ESTABLISHED || sm.currentState == CLOSE_WAIT
}

// CanReceiveData проверяет, можно ли получать данные в текущем состоянии
func (sm *TCPStateMachine) CanReceiveData() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.currentState == ESTABLISHED ||
		sm.currentState == FIN_WAIT_1 ||
		sm.currentState == FIN_WAIT_2
}
