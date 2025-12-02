package tcpconn_test

import (
	"fmt"
	"log"
	"tcpconn"
)

// Пример базового использования RingBuffer
func ExampleRingBuffer_basic() {
	// Создаем буфер емкостью 100 байт
	rb, err := tcpconn.NewRingBuffer(100)
	if err != nil {
		log.Fatal(err)
	}

	// Записываем данные
	data := []byte("Hello, World!")
	n, err := rb.Write(data)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Записано: %d байт\n", n)

	// Читаем данные
	buf := make([]byte, 13)
	n, err = rb.Read(buf)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Прочитано: %s\n", string(buf[:n]))

	// Output:
	// Записано: 13 байт
	// Прочитано: Hello, World!
}

// Пример использования Peek и Skip
func ExampleRingBuffer_peekAndSkip() {
	rb, _ := tcpconn.NewRingBuffer(100)
	rb.Write([]byte("Hello"))

	// Читаем без удаления
	buf := make([]byte, 5)
	n, _ := rb.Peek(buf)
	fmt.Printf("Peek: %s\n", string(buf[:n]))
	fmt.Printf("Доступно после Peek: %d\n", rb.Available())

	// Пропускаем 2 байта
	rb.Skip(2)

	// Читаем оставшиеся
	n, _ = rb.Read(buf)
	fmt.Printf("Read: %s\n", string(buf[:n]))

	// Output:
	// Peek: Hello
	// Доступно после Peek: 5
	// Read: llo
}

// Пример проверки состояния буфера
func ExampleRingBuffer_state() {
	rb, _ := tcpconn.NewRingBuffer(10)

	fmt.Printf("Пустой: %v\n", rb.IsEmpty())
	fmt.Printf("Полный: %v\n", rb.IsFull())

	rb.Write([]byte("1234567890"))

	fmt.Printf("Пустой: %v\n", rb.IsEmpty())
	fmt.Printf("Полный: %v\n", rb.IsFull())
	fmt.Printf("Доступно: %d\n", rb.Available())

	// Output:
	// Пустой: true
	// Полный: false
	// Пустой: false
	// Полный: true
	// Доступно: 10
}

// Пример установки TCP соединения (клиент)
func ExampleTCPStateMachine_clientHandshake() {
	sm := tcpconn.NewTCPStateMachine()

	fmt.Printf("Начальное состояние: %s\n", sm.GetState())

	// Клиент инициирует соединение
	sm.ProcessEvent(tcpconn.ACTIVE_OPEN)
	fmt.Printf("После ACTIVE_OPEN: %s\n", sm.GetState())

	// Получен SYN-ACK от сервера
	sm.ProcessEvent(tcpconn.SYN_ACK)
	fmt.Printf("После SYN_ACK: %s\n", sm.GetState())
	fmt.Printf("Соединение установлено: %v\n", sm.IsConnected())

	// Output:
	// Начальное состояние: CLOSED
	// После ACTIVE_OPEN: SYN_SENT
	// После SYN_ACK: ESTABLISHED
	// Соединение установлено: true
}

// Пример установки TCP соединения (сервер)
func ExampleTCPStateMachine_serverHandshake() {
	sm := tcpconn.NewTCPStateMachine()

	// Сервер слушает входящие соединения
	sm.ProcessEvent(tcpconn.PASSIVE_OPEN)
	fmt.Printf("После PASSIVE_OPEN: %s\n", sm.GetState())

	// Получен SYN от клиента
	sm.ProcessEvent(tcpconn.SYN)
	fmt.Printf("После SYN: %s\n", sm.GetState())

	// Получен ACK от клиента
	sm.ProcessEvent(tcpconn.ACK)
	fmt.Printf("После ACK: %s\n", sm.GetState())
	fmt.Printf("Соединение установлено: %v\n", sm.IsConnected())

	// Output:
	// После PASSIVE_OPEN: LISTEN
	// После SYN: SYN_RECEIVED
	// После ACK: ESTABLISHED
	// Соединение установлено: true
}

// Пример активного закрытия соединения
func ExampleTCPStateMachine_activeClose() {
	sm := tcpconn.NewTCPStateMachine()

	// Устанавливаем соединение
	sm.ProcessEvent(tcpconn.ACTIVE_OPEN)
	sm.ProcessEvent(tcpconn.SYN_ACK)
	fmt.Printf("Соединение: %s\n", sm.GetState())

	// Активное закрытие
	sm.ProcessEvent(tcpconn.CLOSE)
	fmt.Printf("После CLOSE: %s\n", sm.GetState())

	sm.ProcessEvent(tcpconn.ACK)
	fmt.Printf("После ACK: %s\n", sm.GetState())

	sm.ProcessEvent(tcpconn.FIN)
	fmt.Printf("После FIN: %s\n", sm.GetState())

	sm.ProcessEvent(tcpconn.TIMEOUT)
	fmt.Printf("После TIMEOUT: %s\n", sm.GetState())

	// Output:
	// Соединение: ESTABLISHED
	// После CLOSE: FIN_WAIT_1
	// После ACK: FIN_WAIT_2
	// После FIN: TIME_WAIT
	// После TIMEOUT: CLOSED
}

// Пример пассивного закрытия соединения
func ExampleTCPStateMachine_passiveClose() {
	sm := tcpconn.NewTCPStateMachine()

	// Устанавливаем соединение
	sm.ProcessEvent(tcpconn.ACTIVE_OPEN)
	sm.ProcessEvent(tcpconn.SYN_ACK)

	// Получен FIN от удаленной стороны
	sm.ProcessEvent(tcpconn.FIN)
	fmt.Printf("После получения FIN: %s\n", sm.GetState())

	// Локальное закрытие
	sm.ProcessEvent(tcpconn.CLOSE)
	fmt.Printf("После CLOSE: %s\n", sm.GetState())

	// Получен ACK
	sm.ProcessEvent(tcpconn.ACK)
	fmt.Printf("После ACK: %s\n", sm.GetState())

	// Output:
	// После получения FIN: CLOSE_WAIT
	// После CLOSE: LAST_ACK
	// После ACK: CLOSED
}

// Пример использования callbacks
func ExampleTCPStateMachine_callbacks() {
	sm := tcpconn.NewTCPStateMachine()

	// Устанавливаем callback для изменения состояния
	sm.SetStateChangeCallback(func(oldState, newState tcpconn.TCPState, event tcpconn.TCPEvent) {
		fmt.Printf("Переход: %s -> %s\n", oldState, newState)
	})

	// Устанавливаем callback для ошибок
	sm.SetErrorCallback(func(state tcpconn.TCPState, event tcpconn.TCPEvent, err error) {
		fmt.Printf("Ошибка: %v\n", err)
	})

	sm.ProcessEvent(tcpconn.ACTIVE_OPEN)
	sm.ProcessEvent(tcpconn.SYN_ACK)
	sm.ProcessEvent(tcpconn.FIN) // недопустимое событие

	// Output:
	// Переход: CLOSED -> SYN_SENT
	// Переход: SYN_SENT -> ESTABLISHED
	// Переход: ESTABLISHED -> CLOSE_WAIT
}

// Пример проверки возможности отправки/приема данных
func ExampleTCPStateMachine_dataTransfer() {
	sm := tcpconn.NewTCPStateMachine()

	// До установки соединения
	fmt.Printf("Можно отправлять (CLOSED): %v\n", sm.CanSendData())
	fmt.Printf("Можно получать (CLOSED): %v\n", sm.CanReceiveData())

	// Устанавливаем соединение
	sm.ProcessEvent(tcpconn.ACTIVE_OPEN)
	sm.ProcessEvent(tcpconn.SYN_ACK)

	// После установки соединения
	fmt.Printf("Можно отправлять (ESTABLISHED): %v\n", sm.CanSendData())
	fmt.Printf("Можно получать (ESTABLISHED): %v\n", sm.CanReceiveData())

	// Начинаем закрытие
	sm.ProcessEvent(tcpconn.CLOSE)
	fmt.Printf("Можно отправлять (FIN_WAIT_1): %v\n", sm.CanSendData())
	fmt.Printf("Можно получать (FIN_WAIT_1): %v\n", sm.CanReceiveData())

	// Output:
	// Можно отправлять (CLOSED): false
	// Можно получать (CLOSED): false
	// Можно отправлять (ESTABLISHED): true
	// Можно получать (ESTABLISHED): true
	// Можно отправлять (FIN_WAIT_1): false
	// Можно получать (FIN_WAIT_1): true
}

// Пример использования истории переходов
func ExampleTCPStateMachine_history() {
	sm := tcpconn.NewTCPStateMachine()

	// Выполняем несколько переходов
	sm.ProcessEvent(tcpconn.ACTIVE_OPEN)
	sm.ProcessEvent(tcpconn.SYN_ACK)
	sm.ProcessEvent(tcpconn.CLOSE)

	// Получаем историю
	history := sm.GetHistory()
	fmt.Printf("Всего переходов: %d\n", len(history))

	for i, t := range history {
		fmt.Printf("%d: %s -> %s [%s]\n", i+1, t.FromState, t.ToState, t.Event)
	}

	// Output:
	// Всего переходов: 3
	// 1: CLOSED -> SYN_SENT [ACTIVE_OPEN]
	// 2: SYN_SENT -> ESTABLISHED [SYN_ACK]
	// 3: ESTABLISHED -> FIN_WAIT_1 [CLOSE]
}

// Пример сброса соединения с помощью RST
func ExampleTCPStateMachine_reset() {
	sm := tcpconn.NewTCPStateMachine()

	// Устанавливаем соединение
	sm.ProcessEvent(tcpconn.ACTIVE_OPEN)
	sm.ProcessEvent(tcpconn.SYN_ACK)
	fmt.Printf("Состояние: %s\n", sm.GetState())

	// RST сбрасывает соединение из любого состояния
	sm.ProcessEvent(tcpconn.RST)
	fmt.Printf("После RST: %s\n", sm.GetState())

	// Output:
	// Состояние: ESTABLISHED
	// После RST: CLOSED
}

// Пример комбинированного использования
func Example() {
	// Создаем компоненты
	sm := tcpconn.NewTCPStateMachine()
	readBuf, _ := tcpconn.NewRingBuffer(1024)
	writeBuf, _ := tcpconn.NewRingBuffer(1024)

	// Устанавливаем соединение
	sm.ProcessEvent(tcpconn.ACTIVE_OPEN)
	sm.ProcessEvent(tcpconn.SYN_ACK)

	if sm.IsConnected() {
		fmt.Println("Соединение установлено")
	}

	// Записываем данные для отправки
	if sm.CanSendData() {
		writeBuf.Write([]byte("Hello, Server!"))
		fmt.Printf("Данных для отправки: %d байт\n", writeBuf.Available())
	}

	// Симулируем получение данных
	readBuf.Write([]byte("Hello, Client!"))

	// Читаем полученные данные
	if sm.CanReceiveData() {
		buf := make([]byte, 100)
		n, _ := readBuf.Read(buf)
		fmt.Printf("Получено: %s\n", string(buf[:n]))
	}

	// Закрываем соединение
	sm.ProcessEvent(tcpconn.CLOSE)
	fmt.Printf("Соединение закрывается: %v\n", sm.IsClosing())

	// Output:
	// Соединение установлено
	// Данных для отправки: 14 байт
	// Получено: Hello, Client!
	// Соединение закрывается: true
}
