package tcpconn

import (
	"testing"
	"time"
)

func TestTCPConnection_ClientConnect(t *testing.T) {
	conn, err := NewTCPConnection(1024)
	if err != nil {
		t.Fatalf("NewTCPConnection() error = %v", err)
	}

	err = conn.Connect()
	if err != nil {
		t.Errorf("Connect() error = %v", err)
	}

	if !conn.IsConnected() {
		t.Error("IsConnected() = false, want true")
	}

	if conn.GetState() != ESTABLISHED {
		t.Errorf("GetState() = %v, want ESTABLISHED", conn.GetState())
	}
}

func TestTCPConnection_ServerAccept(t *testing.T) {
	conn, err := NewTCPConnection(1024)
	if err != nil {
		t.Fatalf("NewTCPConnection() error = %v", err)
	}

	err = conn.Listen()
	if err != nil {
		t.Errorf("Listen() error = %v", err)
	}

	if conn.GetState() != LISTEN {
		t.Errorf("GetState() = %v, want LISTEN", conn.GetState())
	}

	err = conn.Accept()
	if err != nil {
		t.Errorf("Accept() error = %v", err)
	}

	if !conn.IsConnected() {
		t.Error("IsConnected() = false, want true")
	}
}

func TestTCPConnection_WriteRead(t *testing.T) {
	conn, err := NewTCPConnection(1024)
	if err != nil {
		t.Fatalf("NewTCPConnection() error = %v", err)
	}

	conn.Connect()

	// Запись данных
	testData := []byte("Hello, World!")
	n, err := conn.Write(testData)
	if err != nil {
		t.Errorf("Write() error = %v", err)
	}
	if n != len(testData) {
		t.Errorf("Write() = %v, want %v", n, len(testData))
	}

	// Симулируем получение данных
	conn.readBuffer.Write([]byte("Response"))

	// Чтение данных
	buf := make([]byte, 100)
	n, err = conn.Read(buf)
	if err != nil {
		t.Errorf("Read() error = %v", err)
	}
	if n != 8 {
		t.Errorf("Read() = %v, want 8", n)
	}
}

func TestTCPConnection_Close(t *testing.T) {
	conn, err := NewTCPConnection(1024)
	if err != nil {
		t.Fatalf("NewTCPConnection() error = %v", err)
	}

	conn.Connect()

	err = conn.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// Попытка записи после закрытия
	_, err = conn.Write([]byte("test"))
	if err == nil {
		t.Error("Write() after close should return error")
	}
}

func TestTCPConnection_StateChecks(t *testing.T) {
	conn, err := NewTCPConnection(1024)
	if err != nil {
		t.Fatalf("NewTCPConnection() error = %v", err)
	}

	if conn.IsConnected() {
		t.Error("IsConnected() before connect = true, want false")
	}

	conn.Connect()

	if !conn.IsConnected() {
		t.Error("IsConnected() after connect = false, want true")
	}

	available := conn.AvailableToWrite()
	if available <= 0 {
		t.Errorf("AvailableToWrite() = %v, want > 0", available)
	}
}

func TestMessageProtocol_SendReceive(t *testing.T) {
	mp, err := NewMessageProtocol(4096)
	if err != nil {
		t.Fatalf("NewMessageProtocol() error = %v", err)
	}

	mp.Connect()

	// Отправка сообщения
	testMsg := []byte("Test Message")
	err = mp.SendMessage(testMsg)
	if err != nil {
		t.Errorf("SendMessage() error = %v", err)
	}

	// Симулируем получение: копируем из writeBuffer в readBuffer
	data := mp.conn.writeBuffer.ReadAll()
	mp.conn.readBuffer.Write(data)

	// Получение сообщения
	received, err := mp.ReceiveMessage()
	if err != nil {
		t.Errorf("ReceiveMessage() error = %v", err)
	}

	if string(received) != string(testMsg) {
		t.Errorf("ReceiveMessage() = %s, want %s", received, testMsg)
	}
}

func TestMessageProtocol_EmptyMessage(t *testing.T) {
	mp, err := NewMessageProtocol(4096)
	if err != nil {
		t.Fatalf("NewMessageProtocol() error = %v", err)
	}

	mp.Connect()

	// Отправка пустого сообщения
	err = mp.SendMessage([]byte{})
	if err != nil {
		t.Errorf("SendMessage() error = %v", err)
	}

	// Симулируем получение
	data := mp.conn.writeBuffer.ReadAll()
	mp.conn.readBuffer.Write(data)

	received, err := mp.ReceiveMessage()
	if err != nil {
		t.Errorf("ReceiveMessage() error = %v", err)
	}

	if len(received) != 0 {
		t.Errorf("ReceiveMessage() len = %v, want 0", len(received))
	}
}

func TestStreamProcessor_RegisterHandler(t *testing.T) {
	sp, err := NewStreamProcessor(1024)
	if err != nil {
		t.Fatalf("NewStreamProcessor() error = %v", err)
	}

	called := false
	sp.RegisterHandler(1, func(data []byte) error {
		called = true
		return nil
	})

	// Создаем сообщение: тип(1) + длина(4) + данные
	msg := make([]byte, 9)
	msg[0] = 1 // тип
	msg[1] = 0
	msg[2] = 0
	msg[3] = 0
	msg[4] = 4 // длина = 4
	copy(msg[5:], []byte("test"))

	err = sp.ProcessData(msg)
	if err != nil {
		t.Errorf("ProcessData() error = %v", err)
	}

	if !called {
		t.Error("Handler was not called")
	}
}

func TestStreamProcessor_MultipleMessages(t *testing.T) {
	sp, err := NewStreamProcessor(1024)
	if err != nil {
		t.Fatalf("NewStreamProcessor() error = %v", err)
	}

	count := 0
	sp.RegisterHandler(1, func(data []byte) error {
		count++
		return nil
	})

	// Создаем два сообщения
	msg1 := make([]byte, 9)
	msg1[0] = 1
	msg1[4] = 4
	copy(msg1[5:], []byte("msg1"))

	msg2 := make([]byte, 9)
	msg2[0] = 1
	msg2[4] = 4
	copy(msg2[5:], []byte("msg2"))

	// Обрабатываем оба сообщения
	combined := append(msg1, msg2...)
	err = sp.ProcessData(combined)
	if err != nil {
		t.Errorf("ProcessData() error = %v", err)
	}

	if count != 2 {
		t.Errorf("Handler called %v times, want 2", count)
	}
}

func TestStreamProcessor_PartialMessage(t *testing.T) {
	sp, err := NewStreamProcessor(1024)
	if err != nil {
		t.Fatalf("NewStreamProcessor() error = %v", err)
	}

	called := false
	sp.RegisterHandler(1, func(data []byte) error {
		called = true
		return nil
	})

	// Создаем сообщение и разделяем его
	msg := make([]byte, 9)
	msg[0] = 1
	msg[4] = 4
	copy(msg[5:], []byte("test"))

	// Обрабатываем первую часть (только заголовок)
	err = sp.ProcessData(msg[:5])
	if err != nil {
		t.Errorf("ProcessData() error = %v", err)
	}

	if called {
		t.Error("Handler called with partial message")
	}

	// Обрабатываем остаток
	err = sp.ProcessData(msg[5:])
	if err != nil {
		t.Errorf("ProcessData() error = %v", err)
	}

	if !called {
		t.Error("Handler not called after complete message")
	}
}

func TestConnectionPool_AcquireRelease(t *testing.T) {
	pool, err := NewConnectionPool(3, 1024)
	if err != nil {
		t.Fatalf("NewConnectionPool() error = %v", err)
	}
	defer pool.Close()

	// Получаем соединение
	conn1, err := pool.Acquire()
	if err != nil {
		t.Errorf("Acquire() error = %v", err)
	}

	if conn1 == nil {
		t.Fatal("Acquire() returned nil")
	}

	// Получаем еще одно
	conn2, err := pool.Acquire()
	if err != nil {
		t.Errorf("Acquire() error = %v", err)
	}

	if conn2 == nil {
		t.Fatal("Acquire() returned nil")
	}

	if conn1 == conn2 {
		t.Error("Acquire() returned same connection twice")
	}

	// Возвращаем соединение
	err = pool.Release(conn1)
	if err != nil {
		t.Errorf("Release() error = %v", err)
	}

	// Получаем снова (должны получить то же самое)
	conn3, err := pool.Acquire()
	if err != nil {
		t.Errorf("Acquire() error = %v", err)
	}

	if conn3 != conn1 {
		t.Error("Acquire() after release should return same connection")
	}
}

func TestConnectionPool_MaxSize(t *testing.T) {
	pool, err := NewConnectionPool(2, 1024)
	if err != nil {
		t.Fatalf("NewConnectionPool() error = %v", err)
	}
	defer pool.Close()

	// Получаем максимальное количество соединений
	conn1, _ := pool.Acquire()
	conn2, _ := pool.Acquire()

	// Попытка получить третье соединение в горутине (должна заблокироваться)
	done := make(chan bool)
	go func() {
		_, err := pool.Acquire()
		if err != nil {
			t.Errorf("Acquire() error = %v", err)
		}
		done <- true
	}()

	// Даем время заблокироваться
	time.Sleep(50 * time.Millisecond)

	select {
	case <-done:
		t.Error("Acquire() should block when pool is exhausted")
	default:
		// Ожидаемое поведение
	}

	// Освобождаем соединение
	pool.Release(conn1)

	// Теперь третий Acquire должен завершиться
	select {
	case <-done:
		// Ожидаемое поведение
	case <-time.After(100 * time.Millisecond):
		t.Error("Acquire() did not complete after Release()")
	}

	pool.Release(conn2)
}

func TestConnectionPool_Close(t *testing.T) {
	pool, err := NewConnectionPool(2, 1024)
	if err != nil {
		t.Fatalf("NewConnectionPool() error = %v", err)
	}

	conn1, _ := pool.Acquire()
	conn2, _ := pool.Acquire()

	// Устанавливаем соединения перед закрытием
	conn1.Connect()
	conn2.Connect()

	// Закрываем пул
	err = pool.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// Проверяем, что соединения закрыты
	if !conn1.closed {
		t.Error("Connection 1 not closed")
	}
	if !conn2.closed {
		t.Error("Connection 2 not closed")
	}
}

func BenchmarkTCPConnection_Write(b *testing.B) {
	conn, _ := NewTCPConnection(4096)
	conn.Connect()
	data := []byte("test data")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		conn.Write(data)
		if conn.writeBuffer.IsFull() {
			conn.writeBuffer.Reset()
		}
	}
}

func BenchmarkTCPConnection_Read(b *testing.B) {
	conn, _ := NewTCPConnection(4096)
	conn.Connect()
	buf := make([]byte, 100)

	// Заполняем буфер
	for i := 0; i < 100; i++ {
		conn.readBuffer.Write([]byte("test data"))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := conn.Read(buf)
		if err != nil {
			// Заполняем снова
			for j := 0; j < 10; j++ {
				conn.readBuffer.Write([]byte("test data"))
			}
		}
	}
}

func BenchmarkMessageProtocol_SendReceive(b *testing.B) {
	mp, _ := NewMessageProtocol(8192)
	mp.Connect()
	testMsg := []byte("Benchmark message")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mp.SendMessage(testMsg)
		data := mp.conn.writeBuffer.ReadAll()
		mp.conn.readBuffer.Write(data)
		mp.ReceiveMessage()
	}
}

func BenchmarkStreamProcessor_ProcessData(b *testing.B) {
	sp, _ := NewStreamProcessor(4096)
	sp.RegisterHandler(1, func(data []byte) error {
		return nil
	})

	msg := make([]byte, 9)
	msg[0] = 1
	msg[4] = 4
	copy(msg[5:], []byte("test"))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sp.ProcessData(msg)
	}
}
