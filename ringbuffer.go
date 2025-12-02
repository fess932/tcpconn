package tcpconn

import (
	"errors"
	"sync"
)

var (
	// ErrBufferFull возвращается при попытке записи в полный буфер
	ErrBufferFull = errors.New("buffer is full")
	// ErrBufferEmpty возвращается при попытке чтения из пустого буфера
	ErrBufferEmpty = errors.New("buffer is empty")
	// ErrInvalidCapacity возвращается при создании буфера с нулевой емкостью
	ErrInvalidCapacity = errors.New("capacity must be greater than zero")
	// ErrInvalidSize возвращается при запросе недопустимого размера
	ErrInvalidSize = errors.New("invalid size")
)

// RingBuffer представляет потокобезопасный кольцевой буфер
type RingBuffer struct {
	buffer   []byte
	capacity int
	size     int
	head     int // позиция для записи
	tail     int // позиция для чтения
	mu       sync.Mutex
}

// NewRingBuffer создает новый кольцевой буфер с заданной емкостью
func NewRingBuffer(capacity int) (*RingBuffer, error) {
	if capacity <= 0 {
		return nil, ErrInvalidCapacity
	}

	return &RingBuffer{
		buffer:   make([]byte, capacity),
		capacity: capacity,
		size:     0,
		head:     0,
		tail:     0,
	}, nil
}

// Write записывает данные в буфер
// Возвращает количество записанных байт
func (rb *RingBuffer) Write(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}

	rb.mu.Lock()
	defer rb.mu.Unlock()

	availableSpace := rb.capacity - rb.size
	if availableSpace == 0 {
		return 0, ErrBufferFull
	}

	toWrite := len(data)
	if toWrite > availableSpace {
		toWrite = availableSpace
	}

	for i := 0; i < toWrite; i++ {
		rb.buffer[rb.head] = data[i]
		rb.head = (rb.head + 1) % rb.capacity
	}

	rb.size += toWrite
	return toWrite, nil
}

// WriteAll записывает все данные в буфер или возвращает ошибку
func (rb *RingBuffer) WriteAll(data []byte) error {
	if len(data) == 0 {
		return nil
	}

	rb.mu.Lock()
	defer rb.mu.Unlock()

	availableSpace := rb.capacity - rb.size
	if len(data) > availableSpace {
		return ErrBufferFull
	}

	for i := 0; i < len(data); i++ {
		rb.buffer[rb.head] = data[i]
		rb.head = (rb.head + 1) % rb.capacity
	}

	rb.size += len(data)
	return nil
}

// Read читает данные из буфера
// Возвращает количество прочитанных байт
func (rb *RingBuffer) Read(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}

	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.size == 0 {
		return 0, ErrBufferEmpty
	}

	toRead := len(data)
	if toRead > rb.size {
		toRead = rb.size
	}

	for i := 0; i < toRead; i++ {
		data[i] = rb.buffer[rb.tail]
		rb.tail = (rb.tail + 1) % rb.capacity
	}

	rb.size -= toRead
	return toRead, nil
}

// ReadAll читает все доступные данные из буфера
func (rb *RingBuffer) ReadAll() []byte {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.size == 0 {
		return nil
	}

	data := make([]byte, rb.size)
	for i := 0; i < rb.size; i++ {
		data[i] = rb.buffer[rb.tail]
		rb.tail = (rb.tail + 1) % rb.capacity
	}

	rb.size = 0
	return data
}

// Peek читает данные без удаления их из буфера
func (rb *RingBuffer) Peek(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}

	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.size == 0 {
		return 0, ErrBufferEmpty
	}

	toRead := len(data)
	if toRead > rb.size {
		toRead = rb.size
	}

	tail := rb.tail
	for i := 0; i < toRead; i++ {
		data[i] = rb.buffer[tail]
		tail = (tail + 1) % rb.capacity
	}

	return toRead, nil
}

// Skip пропускает n байт в буфере
func (rb *RingBuffer) Skip(n int) error {
	if n < 0 {
		return ErrInvalidSize
	}

	rb.mu.Lock()
	defer rb.mu.Unlock()

	if n > rb.size {
		return ErrBufferEmpty
	}

	rb.tail = (rb.tail + n) % rb.capacity
	rb.size -= n
	return nil
}

// Available возвращает количество байт доступных для чтения
func (rb *RingBuffer) Available() int {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	return rb.size
}

// FreeSpace возвращает количество свободного места в буфере
func (rb *RingBuffer) FreeSpace() int {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	return rb.capacity - rb.size
}

// Capacity возвращает емкость буфера
func (rb *RingBuffer) Capacity() int {
	return rb.capacity
}

// IsEmpty проверяет, пуст ли буфер
func (rb *RingBuffer) IsEmpty() bool {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	return rb.size == 0
}

// IsFull проверяет, заполнен ли буфер
func (rb *RingBuffer) IsFull() bool {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	return rb.size == rb.capacity
}

// Reset очищает буфер
func (rb *RingBuffer) Reset() {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.size = 0
	rb.head = 0
	rb.tail = 0
}
