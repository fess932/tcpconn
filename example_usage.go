package tcpconn

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"
)

// TCPConnection представляет TCP соединение с управлением состоянием и буферами
type TCPConnection struct {
	state       *TCPStateMachine
	readBuffer  *RingBuffer
	writeBuffer *RingBuffer
	stats       *Statistics
	mu          sync.RWMutex
	closed      bool
}

// NewTCPConnection создает новое TCP соединение
func NewTCPConnection(bufferSize int) (*TCPConnection, error) {
	return NewTCPConnectionWithStats(bufferSize, nil)
}

// NewTCPConnectionWithStats создает новое TCP соединение с возможностью передать свой объект Statistics.
// Если stats == nil, создается новый объект статистики.
// Это позволяет разделять статистику между несколькими соединениями или управлять ей извне.
func NewTCPConnectionWithStats(bufferSize int, stats *Statistics) (*TCPConnection, error) {
	if bufferSize <= 0 {
		bufferSize = 4096
	}

	readBuf, err := NewRingBuffer(bufferSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create read buffer: %w", err)
	}

	writeBuf, err := NewRingBuffer(bufferSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create write buffer: %w", err)
	}

	if stats == nil {
		stats = NewStatistics()
	}

	return &TCPConnection{
		state:       NewTCPStateMachine(),
		readBuffer:  readBuf,
		writeBuffer: writeBuf,
		stats:       stats,
		closed:      false,
	}, nil
}

// Connect выполняет активное открытие соединения (клиент)
func (c *TCPConnection) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.state.ProcessEvent(ACTIVE_OPEN); err != nil {
		return fmt.Errorf("connect failed: %w", err)
	}

	// Симуляция получения SYN-ACK
	if err := c.state.ProcessEvent(SYN_ACK); err != nil {
		return fmt.Errorf("connect failed: %w", err)
	}

	return nil
}

// Listen выполняет пассивное открытие соединения (сервер)
func (c *TCPConnection) Listen() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.state.ProcessEvent(PASSIVE_OPEN); err != nil {
		return fmt.Errorf("listen failed: %w", err)
	}

	return nil
}

// Accept принимает входящее соединение (сервер)
func (c *TCPConnection) Accept() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Получен SYN
	if err := c.state.ProcessEvent(SYN); err != nil {
		return fmt.Errorf("accept failed: %w", err)
	}

	// Получен ACK
	if err := c.state.ProcessEvent(ACK); err != nil {
		return fmt.Errorf("accept failed: %w", err)
	}

	return nil
}

// Write записывает данные в буфер отправки
func (c *TCPConnection) Write(data []byte) (int, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		c.stats.RecordError()
		return 0, io.ErrClosedPipe
	}

	if !c.state.CanSendData() {
		c.stats.RecordError()
		return 0, fmt.Errorf("cannot send data in state %s", c.state.GetState())
	}

	n, err := c.writeBuffer.Write(data)
	if err == nil {
		c.stats.RecordPacketSent(uint64(n))
	} else {
		c.stats.RecordError()
	}
	return n, err
}

// Read читает данные из буфера приема
func (c *TCPConnection) Read(buf []byte) (int, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed && c.readBuffer.IsEmpty() {
		return 0, io.EOF
	}

	if !c.state.CanReceiveData() && c.readBuffer.IsEmpty() {
		c.stats.RecordError()
		return 0, fmt.Errorf("cannot receive data in state %s", c.state.GetState())
	}

	n, err := c.readBuffer.Read(buf)
	if err == nil && n > 0 {
		c.stats.RecordPacketReceived(uint64(n))
	} else if err != nil {
		c.stats.RecordError()
	}
	return n, err
}

// Close закрывает соединение
func (c *TCPConnection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	if err := c.state.ProcessEvent(CLOSE); err != nil {
		return fmt.Errorf("close failed: %w", err)
	}

	c.closed = true
	return nil
}

// GetState возвращает текущее состояние соединения
func (c *TCPConnection) GetState() TCPState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state.GetState()
}

// IsConnected проверяет, установлено ли соединение
func (c *TCPConnection) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state.IsConnected()
}

// AvailableToRead возвращает количество байт доступных для чтения
func (c *TCPConnection) AvailableToRead() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.readBuffer.Available()
}

// AvailableToWrite возвращает свободное место в буфере отправки
func (c *TCPConnection) AvailableToWrite() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.writeBuffer.FreeSpace()
}

// GetStatisticsSnapshot возвращает снимок статистики
func (c *TCPConnection) GetStatisticsSnapshot() Snapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.stats.GetSnapshot()
}

// ResetStatistics сбрасывает статистику соединения
func (c *TCPConnection) ResetStatistics() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stats.Reset()
}

// MessageProtocol представляет протокол с длиной сообщения
type MessageProtocol struct {
	conn *TCPConnection
}

// NewMessageProtocol создает новый протокол сообщений
func NewMessageProtocol(bufferSize int) (*MessageProtocol, error) {
	conn, err := NewTCPConnection(bufferSize)
	if err != nil {
		return nil, err
	}

	return &MessageProtocol{
		conn: conn,
	}, nil
}

// SendMessage отправляет сообщение с заголовком длины (4 байта)
func (mp *MessageProtocol) SendMessage(data []byte) error {
	if len(data) > 0xFFFFFFFF {
		return errors.New("message too large")
	}

	// Создаем буфер с заголовком
	header := make([]byte, 4)
	binary.BigEndian.PutUint32(header, uint32(len(data)))

	// Записываем заголовок
	if _, err := mp.conn.Write(header); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	// Записываем данные
	if _, err := mp.conn.Write(data); err != nil {
		return fmt.Errorf("failed to write data: %w", err)
	}

	return nil
}

// ReceiveMessage получает сообщение (блокируется до получения полного сообщения)
func (mp *MessageProtocol) ReceiveMessage() ([]byte, error) {
	// Читаем заголовок (4 байта)
	header := make([]byte, 4)
	totalRead := 0

	for totalRead < 4 {
		n, err := mp.conn.Read(header[totalRead:])
		if err != nil {
			return nil, err
		}
		totalRead += n
		if totalRead < 4 {
			time.Sleep(10 * time.Millisecond) // Небольшая задержка
		}
	}

	// Декодируем длину
	length := binary.BigEndian.Uint32(header)
	if length == 0 {
		return []byte{}, nil
	}

	// Читаем данные
	data := make([]byte, length)
	totalRead = 0

	for totalRead < int(length) {
		n, err := mp.conn.Read(data[totalRead:])
		if err != nil {
			return nil, err
		}
		totalRead += n
		if totalRead < int(length) {
			time.Sleep(10 * time.Millisecond)
		}
	}

	return data, nil
}

// Connect устанавливает соединение
func (mp *MessageProtocol) Connect() error {
	return mp.conn.Connect()
}

// Close закрывает соединение
func (mp *MessageProtocol) Close() error {
	return mp.conn.Close()
}

// StreamProcessor обрабатывает поток данных
type StreamProcessor struct {
	buffer    *RingBuffer
	callbacks map[byte]func([]byte) error
	mu        sync.RWMutex
}

// NewStreamProcessor создает новый обработчик потока
func NewStreamProcessor(bufferSize int) (*StreamProcessor, error) {
	buffer, err := NewRingBuffer(bufferSize)
	if err != nil {
		return nil, err
	}

	return &StreamProcessor{
		buffer:    buffer,
		callbacks: make(map[byte]func([]byte) error),
	}, nil
}

// RegisterHandler регистрирует обработчик для типа сообщения
func (sp *StreamProcessor) RegisterHandler(msgType byte, handler func([]byte) error) {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	sp.callbacks[msgType] = handler
}

// ProcessData добавляет данные в буфер и обрабатывает их
func (sp *StreamProcessor) ProcessData(data []byte) error {
	if _, err := sp.buffer.Write(data); err != nil {
		return fmt.Errorf("failed to write to buffer: %w", err)
	}

	return sp.processMessages()
}

// processMessages обрабатывает все доступные сообщения в буфере
func (sp *StreamProcessor) processMessages() error {
	for {
		// Проверяем, есть ли минимум данных для заголовка (тип + длина = 5 байт)
		if sp.buffer.Available() < 5 {
			break
		}

		// Читаем заголовок без удаления
		header := make([]byte, 5)
		if _, err := sp.buffer.Peek(header); err != nil {
			return err
		}

		msgType := header[0]
		length := binary.BigEndian.Uint32(header[1:5])

		// Проверяем, есть ли полное сообщение
		if sp.buffer.Available() < int(5+length) {
			break
		}

		// Пропускаем заголовок
		sp.buffer.Skip(5)

		// Читаем данные сообщения
		msgData := make([]byte, length)
		if _, err := sp.buffer.Read(msgData); err != nil {
			return fmt.Errorf("failed to read message data: %w", err)
		}

		// Обрабатываем сообщение
		sp.mu.RLock()
		handler, exists := sp.callbacks[msgType]
		sp.mu.RUnlock()

		if exists {
			if err := handler(msgData); err != nil {
				return fmt.Errorf("handler error for type %d: %w", msgType, err)
			}
		}
	}

	return nil
}

// ConnectionPool управляет пулом соединений
type ConnectionPool struct {
	connections []*TCPConnection
	available   chan int
	mu          sync.Mutex
	maxSize     int
	bufferSize  int
}

// NewConnectionPool создает новый пул соединений
func NewConnectionPool(maxSize, bufferSize int) (*ConnectionPool, error) {
	if maxSize <= 0 {
		return nil, errors.New("pool size must be positive")
	}

	pool := &ConnectionPool{
		connections: make([]*TCPConnection, 0, maxSize),
		available:   make(chan int, maxSize),
		maxSize:     maxSize,
		bufferSize:  bufferSize,
	}

	return pool, nil
}

// Acquire получает соединение из пула
func (cp *ConnectionPool) Acquire() (*TCPConnection, error) {
	cp.mu.Lock()

	// Если есть доступное соединение, используем его
	select {
	case idx := <-cp.available:
		conn := cp.connections[idx]
		cp.mu.Unlock()
		return conn, nil
	default:
		// Если пул не заполнен, создаем новое соединение
		if len(cp.connections) < cp.maxSize {
			conn, err := NewTCPConnection(cp.bufferSize)
			if err != nil {
				cp.mu.Unlock()
				return nil, err
			}
			cp.connections = append(cp.connections, conn)
			cp.mu.Unlock()
			return conn, nil
		}
	}

	cp.mu.Unlock()

	// Ждем доступного соединения
	idx := <-cp.available
	cp.mu.Lock()
	conn := cp.connections[idx]
	cp.mu.Unlock()
	return conn, nil
}

// Release возвращает соединение в пул
func (cp *ConnectionPool) Release(conn *TCPConnection) error {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	// Находим индекс соединения
	for i, c := range cp.connections {
		if c == conn {
			select {
			case cp.available <- i:
				return nil
			default:
				return errors.New("failed to return connection to pool")
			}
		}
	}

	return errors.New("connection not found in pool")
}

// Close закрывает все соединения в пуле
func (cp *ConnectionPool) Close() error {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	var lastErr error
	for _, conn := range cp.connections {
		if err := conn.Close(); err != nil {
			lastErr = err
		}
	}

	close(cp.available)
	return lastErr
}
