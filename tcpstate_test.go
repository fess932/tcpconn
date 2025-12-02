package tcpconn

import (
	"testing"
)

func TestNewTCPStateMachine(t *testing.T) {
	sm := NewTCPStateMachine()
	if sm == nil {
		t.Fatal("NewTCPStateMachine() returned nil")
	}
	if sm.GetState() != CLOSED {
		t.Errorf("GetState() = %v, want CLOSED", sm.GetState())
	}
	if !sm.IsClosed() {
		t.Error("IsClosed() = false, want true")
	}
}

func TestTCPState_String(t *testing.T) {
	tests := []struct {
		state TCPState
		want  string
	}{
		{CLOSED, "CLOSED"},
		{LISTEN, "LISTEN"},
		{SYN_SENT, "SYN_SENT"},
		{SYN_RECEIVED, "SYN_RECEIVED"},
		{ESTABLISHED, "ESTABLISHED"},
		{FIN_WAIT_1, "FIN_WAIT_1"},
		{FIN_WAIT_2, "FIN_WAIT_2"},
		{CLOSE_WAIT, "CLOSE_WAIT"},
		{CLOSING, "CLOSING"},
		{LAST_ACK, "LAST_ACK"},
		{TIME_WAIT, "TIME_WAIT"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.state.String(); got != tt.want {
				t.Errorf("String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTCPEvent_String(t *testing.T) {
	tests := []struct {
		event TCPEvent
		want  string
	}{
		{PASSIVE_OPEN, "PASSIVE_OPEN"},
		{ACTIVE_OPEN, "ACTIVE_OPEN"},
		{SYN, "SYN"},
		{SYN_ACK, "SYN_ACK"},
		{ACK, "ACK"},
		{FIN, "FIN"},
		{FIN_ACK, "FIN_ACK"},
		{CLOSE, "CLOSE"},
		{TIMEOUT, "TIMEOUT"},
		{RST, "RST"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.event.String(); got != tt.want {
				t.Errorf("String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTCPStateMachine_ClientHandshake(t *testing.T) {
	sm := NewTCPStateMachine()

	// Клиент инициирует соединение
	err := sm.ProcessEvent(ACTIVE_OPEN)
	if err != nil {
		t.Errorf("ProcessEvent(ACTIVE_OPEN) error = %v", err)
	}
	if sm.GetState() != SYN_SENT {
		t.Errorf("GetState() = %v, want SYN_SENT", sm.GetState())
	}

	// Клиент получает SYN-ACK
	err = sm.ProcessEvent(SYN_ACK)
	if err != nil {
		t.Errorf("ProcessEvent(SYN_ACK) error = %v", err)
	}
	if sm.GetState() != ESTABLISHED {
		t.Errorf("GetState() = %v, want ESTABLISHED", sm.GetState())
	}

	if !sm.IsConnected() {
		t.Error("IsConnected() = false, want true")
	}
}

func TestTCPStateMachine_ServerHandshake(t *testing.T) {
	sm := NewTCPStateMachine()

	// Сервер открывает пассивное соединение
	err := sm.ProcessEvent(PASSIVE_OPEN)
	if err != nil {
		t.Errorf("ProcessEvent(PASSIVE_OPEN) error = %v", err)
	}
	if sm.GetState() != LISTEN {
		t.Errorf("GetState() = %v, want LISTEN", sm.GetState())
	}

	// Сервер получает SYN
	err = sm.ProcessEvent(SYN)
	if err != nil {
		t.Errorf("ProcessEvent(SYN) error = %v", err)
	}
	if sm.GetState() != SYN_RECEIVED {
		t.Errorf("GetState() = %v, want SYN_RECEIVED", sm.GetState())
	}

	// Сервер получает ACK
	err = sm.ProcessEvent(ACK)
	if err != nil {
		t.Errorf("ProcessEvent(ACK) error = %v", err)
	}
	if sm.GetState() != ESTABLISHED {
		t.Errorf("GetState() = %v, want ESTABLISHED", sm.GetState())
	}

	if !sm.IsConnected() {
		t.Error("IsConnected() = false, want true")
	}
}

func TestTCPStateMachine_ActiveClose(t *testing.T) {
	sm := NewTCPStateMachine()

	// Установка соединения
	sm.ProcessEvent(ACTIVE_OPEN)
	sm.ProcessEvent(SYN_ACK)

	// Активная сторона закрывает соединение
	err := sm.ProcessEvent(CLOSE)
	if err != nil {
		t.Errorf("ProcessEvent(CLOSE) error = %v", err)
	}
	if sm.GetState() != FIN_WAIT_1 {
		t.Errorf("GetState() = %v, want FIN_WAIT_1", sm.GetState())
	}

	// Получаем ACK
	err = sm.ProcessEvent(ACK)
	if err != nil {
		t.Errorf("ProcessEvent(ACK) error = %v", err)
	}
	if sm.GetState() != FIN_WAIT_2 {
		t.Errorf("GetState() = %v, want FIN_WAIT_2", sm.GetState())
	}

	// Получаем FIN
	err = sm.ProcessEvent(FIN)
	if err != nil {
		t.Errorf("ProcessEvent(FIN) error = %v", err)
	}
	if sm.GetState() != TIME_WAIT {
		t.Errorf("GetState() = %v, want TIME_WAIT", sm.GetState())
	}

	// Таймаут
	err = sm.ProcessEvent(TIMEOUT)
	if err != nil {
		t.Errorf("ProcessEvent(TIMEOUT) error = %v", err)
	}
	if sm.GetState() != CLOSED {
		t.Errorf("GetState() = %v, want CLOSED", sm.GetState())
	}
}

func TestTCPStateMachine_PassiveClose(t *testing.T) {
	sm := NewTCPStateMachine()

	// Установка соединения
	sm.ProcessEvent(ACTIVE_OPEN)
	sm.ProcessEvent(SYN_ACK)

	// Пассивная сторона получает FIN
	err := sm.ProcessEvent(FIN)
	if err != nil {
		t.Errorf("ProcessEvent(FIN) error = %v", err)
	}
	if sm.GetState() != CLOSE_WAIT {
		t.Errorf("GetState() = %v, want CLOSE_WAIT", sm.GetState())
	}

	// Пассивная сторона закрывает соединение
	err = sm.ProcessEvent(CLOSE)
	if err != nil {
		t.Errorf("ProcessEvent(CLOSE) error = %v", err)
	}
	if sm.GetState() != LAST_ACK {
		t.Errorf("GetState() = %v, want LAST_ACK", sm.GetState())
	}

	// Получаем ACK
	err = sm.ProcessEvent(ACK)
	if err != nil {
		t.Errorf("ProcessEvent(ACK) error = %v", err)
	}
	if sm.GetState() != CLOSED {
		t.Errorf("GetState() = %v, want CLOSED", sm.GetState())
	}
}

func TestTCPStateMachine_SimultaneousClose(t *testing.T) {
	sm := NewTCPStateMachine()

	// Установка соединения
	sm.ProcessEvent(ACTIVE_OPEN)
	sm.ProcessEvent(SYN_ACK)

	// Активная сторона закрывает
	err := sm.ProcessEvent(CLOSE)
	if err != nil {
		t.Errorf("ProcessEvent(CLOSE) error = %v", err)
	}
	if sm.GetState() != FIN_WAIT_1 {
		t.Errorf("GetState() = %v, want FIN_WAIT_1", sm.GetState())
	}

	// Получаем FIN вместо ACK (одновременное закрытие)
	err = sm.ProcessEvent(FIN)
	if err != nil {
		t.Errorf("ProcessEvent(FIN) error = %v", err)
	}
	if sm.GetState() != CLOSING {
		t.Errorf("GetState() = %v, want CLOSING", sm.GetState())
	}

	// Получаем ACK
	err = sm.ProcessEvent(ACK)
	if err != nil {
		t.Errorf("ProcessEvent(ACK) error = %v", err)
	}
	if sm.GetState() != TIME_WAIT {
		t.Errorf("GetState() = %v, want TIME_WAIT", sm.GetState())
	}
}

func TestTCPStateMachine_Reset(t *testing.T) {
	sm := NewTCPStateMachine()

	// Устанавливаем соединение
	sm.ProcessEvent(ACTIVE_OPEN)
	sm.ProcessEvent(SYN_ACK)

	// RST должен перевести в CLOSED из любого состояния
	err := sm.ProcessEvent(RST)
	if err != nil {
		t.Errorf("ProcessEvent(RST) error = %v", err)
	}
	if sm.GetState() != CLOSED {
		t.Errorf("GetState() = %v, want CLOSED", sm.GetState())
	}
}

func TestTCPStateMachine_InvalidTransition(t *testing.T) {
	sm := NewTCPStateMachine()

	// Попытка недопустимого перехода
	err := sm.ProcessEvent(FIN)
	if err == nil {
		t.Error("ProcessEvent(FIN) from CLOSED should return error")
	}

	// Состояние не должно измениться
	if sm.GetState() != CLOSED {
		t.Errorf("GetState() = %v, want CLOSED after invalid transition", sm.GetState())
	}
}

func TestTCPStateMachine_Callbacks(t *testing.T) {
	sm := NewTCPStateMachine()

	stateChangeCalled := false
	errorCalled := false

	sm.SetStateChangeCallback(func(oldState, newState TCPState, event TCPEvent) {
		stateChangeCalled = true
		if oldState != CLOSED {
			t.Errorf("oldState = %v, want CLOSED", oldState)
		}
		if newState != SYN_SENT {
			t.Errorf("newState = %v, want SYN_SENT", newState)
		}
		if event != ACTIVE_OPEN {
			t.Errorf("event = %v, want ACTIVE_OPEN", event)
		}
	})

	sm.SetErrorCallback(func(state TCPState, event TCPEvent, err error) {
		errorCalled = true
	})

	// Валидный переход
	sm.ProcessEvent(ACTIVE_OPEN)
	if !stateChangeCalled {
		t.Error("StateChangeCallback was not called")
	}

	// Невалидный переход
	sm.ProcessEvent(PASSIVE_OPEN)
	if !errorCalled {
		t.Error("ErrorCallback was not called")
	}
}

func TestTCPStateMachine_History(t *testing.T) {
	sm := NewTCPStateMachine()

	// Выполняем несколько переходов
	sm.ProcessEvent(ACTIVE_OPEN)
	sm.ProcessEvent(SYN_ACK)
	sm.ProcessEvent(CLOSE)

	history := sm.GetHistory()
	if len(history) != 3 {
		t.Errorf("len(history) = %v, want 3", len(history))
	}

	// Проверяем первый переход
	if history[0].FromState != CLOSED {
		t.Errorf("history[0].FromState = %v, want CLOSED", history[0].FromState)
	}
	if history[0].ToState != SYN_SENT {
		t.Errorf("history[0].ToState = %v, want SYN_SENT", history[0].ToState)
	}
	if history[0].Event != ACTIVE_OPEN {
		t.Errorf("history[0].Event = %v, want ACTIVE_OPEN", history[0].Event)
	}

	// Очищаем историю
	sm.ClearHistory()
	history = sm.GetHistory()
	if len(history) != 0 {
		t.Errorf("len(history) after clear = %v, want 0", len(history))
	}
}

func TestTCPStateMachine_ResetMethod(t *testing.T) {
	sm := NewTCPStateMachine()

	// Устанавливаем соединение
	sm.ProcessEvent(ACTIVE_OPEN)
	sm.ProcessEvent(SYN_ACK)

	// Сбрасываем машину состояний
	sm.Reset()

	if sm.GetState() != CLOSED {
		t.Errorf("GetState() after reset = %v, want CLOSED", sm.GetState())
	}

	history := sm.GetHistory()
	if len(history) != 0 {
		t.Errorf("len(history) after reset = %v, want 0", len(history))
	}
}

func TestTCPStateMachine_CanSendData(t *testing.T) {
	sm := NewTCPStateMachine()

	// В CLOSED нельзя отправлять данные
	if sm.CanSendData() {
		t.Error("CanSendData() in CLOSED = true, want false")
	}

	// Устанавливаем соединение
	sm.ProcessEvent(ACTIVE_OPEN)
	sm.ProcessEvent(SYN_ACK)

	// В ESTABLISHED можно отправлять данные
	if !sm.CanSendData() {
		t.Error("CanSendData() in ESTABLISHED = false, want true")
	}

	// Получаем FIN (переходим в CLOSE_WAIT)
	sm.ProcessEvent(FIN)

	// В CLOSE_WAIT еще можно отправлять данные
	if !sm.CanSendData() {
		t.Error("CanSendData() in CLOSE_WAIT = false, want true")
	}

	// Закрываем соединение (переходим в LAST_ACK)
	sm.ProcessEvent(CLOSE)

	// В LAST_ACK нельзя отправлять данные
	if sm.CanSendData() {
		t.Error("CanSendData() in LAST_ACK = true, want false")
	}
}

func TestTCPStateMachine_CanReceiveData(t *testing.T) {
	sm := NewTCPStateMachine()

	// В CLOSED нельзя получать данные
	if sm.CanReceiveData() {
		t.Error("CanReceiveData() in CLOSED = true, want false")
	}

	// Устанавливаем соединение
	sm.ProcessEvent(ACTIVE_OPEN)
	sm.ProcessEvent(SYN_ACK)

	// В ESTABLISHED можно получать данные
	if !sm.CanReceiveData() {
		t.Error("CanReceiveData() in ESTABLISHED = false, want true")
	}

	// Закрываем соединение (переходим в FIN_WAIT_1)
	sm.ProcessEvent(CLOSE)

	// В FIN_WAIT_1 еще можно получать данные
	if !sm.CanReceiveData() {
		t.Error("CanReceiveData() in FIN_WAIT_1 = false, want true")
	}
}

func TestTCPStateMachine_IsClosing(t *testing.T) {
	sm := NewTCPStateMachine()

	// В CLOSED не находится в процессе закрытия
	if sm.IsClosing() {
		t.Error("IsClosing() in CLOSED = true, want false")
	}

	// Устанавливаем соединение
	sm.ProcessEvent(ACTIVE_OPEN)
	sm.ProcessEvent(SYN_ACK)

	// В ESTABLISHED не находится в процессе закрытия
	if sm.IsClosing() {
		t.Error("IsClosing() in ESTABLISHED = true, want false")
	}

	// Закрываем соединение
	sm.ProcessEvent(CLOSE)

	// В FIN_WAIT_1 находится в процессе закрытия
	if !sm.IsClosing() {
		t.Error("IsClosing() in FIN_WAIT_1 = false, want true")
	}
}

func TestTCPStateMachine_Timeout(t *testing.T) {
	sm := NewTCPStateMachine()

	// Клиент инициирует соединение
	sm.ProcessEvent(ACTIVE_OPEN)

	// Таймаут в SYN_SENT
	err := sm.ProcessEvent(TIMEOUT)
	if err != nil {
		t.Errorf("ProcessEvent(TIMEOUT) error = %v", err)
	}
	if sm.GetState() != CLOSED {
		t.Errorf("GetState() = %v, want CLOSED", sm.GetState())
	}
}

func TestTCPStateMachine_FINandACK(t *testing.T) {
	sm := NewTCPStateMachine()

	// Устанавливаем соединение
	sm.ProcessEvent(ACTIVE_OPEN)
	sm.ProcessEvent(SYN_ACK)

	// Закрываем соединение
	sm.ProcessEvent(CLOSE)

	// Получаем FIN_ACK (одновременно FIN и ACK)
	err := sm.ProcessEvent(FIN_ACK)
	if err != nil {
		t.Errorf("ProcessEvent(FIN_ACK) error = %v", err)
	}
	if sm.GetState() != TIME_WAIT {
		t.Errorf("GetState() = %v, want TIME_WAIT", sm.GetState())
	}
}

func BenchmarkTCPStateMachine_ProcessEvent(b *testing.B) {
	sm := NewTCPStateMachine()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sm.Reset()
		sm.ProcessEvent(ACTIVE_OPEN)
		sm.ProcessEvent(SYN_ACK)
		sm.ProcessEvent(CLOSE)
		sm.ProcessEvent(ACK)
		sm.ProcessEvent(FIN)
		sm.ProcessEvent(TIMEOUT)
	}
}
