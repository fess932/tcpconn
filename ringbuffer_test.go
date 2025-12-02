package tcpconn

import (
	"bytes"
	"sync"
	"testing"
)

func TestNewRingBuffer(t *testing.T) {
	tests := []struct {
		name     string
		capacity int
		wantErr  bool
	}{
		{"valid capacity", 10, false},
		{"zero capacity", 0, true},
		{"negative capacity", -1, true},
		{"large capacity", 1024, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rb, err := NewRingBuffer(tt.capacity)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewRingBuffer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if rb.Capacity() != tt.capacity {
					t.Errorf("Capacity() = %v, want %v", rb.Capacity(), tt.capacity)
				}
				if rb.Available() != 0 {
					t.Errorf("Available() = %v, want 0", rb.Available())
				}
				if rb.FreeSpace() != tt.capacity {
					t.Errorf("FreeSpace() = %v, want %v", rb.FreeSpace(), tt.capacity)
				}
			}
		})
	}
}

func TestRingBuffer_Write(t *testing.T) {
	rb, err := NewRingBuffer(10)
	if err != nil {
		t.Fatalf("NewRingBuffer() error = %v", err)
	}

	// Запись данных
	data := []byte("hello")
	n, err := rb.Write(data)
	if err != nil {
		t.Errorf("Write() error = %v", err)
	}
	if n != len(data) {
		t.Errorf("Write() = %v, want %v", n, len(data))
	}
	if rb.Available() != len(data) {
		t.Errorf("Available() = %v, want %v", rb.Available(), len(data))
	}

	// Запись до заполнения
	data2 := []byte("world")
	n, err = rb.Write(data2)
	if err != nil {
		t.Errorf("Write() error = %v", err)
	}
	if n != len(data2) {
		t.Errorf("Write() = %v, want %v", n, len(data2))
	}

	// Попытка записи в полный буфер
	n, err = rb.Write([]byte("x"))
	if err != ErrBufferFull {
		t.Errorf("Write() error = %v, want ErrBufferFull", err)
	}
	if n != 0 {
		t.Errorf("Write() = %v, want 0", n)
	}
}

func TestRingBuffer_WriteAll(t *testing.T) {
	rb, err := NewRingBuffer(10)
	if err != nil {
		t.Fatalf("NewRingBuffer() error = %v", err)
	}

	// Успешная запись
	data := []byte("hello")
	err = rb.WriteAll(data)
	if err != nil {
		t.Errorf("WriteAll() error = %v", err)
	}

	// Попытка записать больше, чем свободного места
	data2 := []byte("world!")
	err = rb.WriteAll(data2)
	if err != ErrBufferFull {
		t.Errorf("WriteAll() error = %v, want ErrBufferFull", err)
	}

	// Проверяем, что частичная запись не произошла
	if rb.Available() != 5 {
		t.Errorf("Available() = %v, want 5", rb.Available())
	}
}

func TestRingBuffer_Read(t *testing.T) {
	rb, err := NewRingBuffer(10)
	if err != nil {
		t.Fatalf("NewRingBuffer() error = %v", err)
	}

	// Запись данных
	original := []byte("hello")
	_, err = rb.Write(original)
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	// Чтение данных
	result := make([]byte, 5)
	n, err := rb.Read(result)
	if err != nil {
		t.Errorf("Read() error = %v", err)
	}
	if n != len(original) {
		t.Errorf("Read() = %v, want %v", n, len(original))
	}
	if !bytes.Equal(result, original) {
		t.Errorf("Read() = %q, want %q", result, original)
	}

	// Чтение из пустого буфера
	n, err = rb.Read(result)
	if err != ErrBufferEmpty {
		t.Errorf("Read() error = %v, want ErrBufferEmpty", err)
	}
	if n != 0 {
		t.Errorf("Read() = %v, want 0", n)
	}
}

func TestRingBuffer_ReadAll(t *testing.T) {
	rb, err := NewRingBuffer(10)
	if err != nil {
		t.Fatalf("NewRingBuffer() error = %v", err)
	}

	original := []byte("hello")
	_, err = rb.Write(original)
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	result := rb.ReadAll()
	if !bytes.Equal(result, original) {
		t.Errorf("ReadAll() = %q, want %q", result, original)
	}
	if rb.Available() != 0 {
		t.Errorf("Available() = %v, want 0", rb.Available())
	}

	// ReadAll из пустого буфера
	result = rb.ReadAll()
	if result != nil {
		t.Errorf("ReadAll() = %v, want nil", result)
	}
}

func TestRingBuffer_Peek(t *testing.T) {
	rb, err := NewRingBuffer(10)
	if err != nil {
		t.Fatalf("NewRingBuffer() error = %v", err)
	}

	original := []byte("hello")
	_, err = rb.Write(original)
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	// Peek не должен удалять данные
	result := make([]byte, 5)
	n, err := rb.Peek(result)
	if err != nil {
		t.Errorf("Peek() error = %v", err)
	}
	if n != len(original) {
		t.Errorf("Peek() = %v, want %v", n, len(original))
	}
	if !bytes.Equal(result, original) {
		t.Errorf("Peek() = %q, want %q", result, original)
	}

	// Данные должны остаться в буфере
	if rb.Available() != len(original) {
		t.Errorf("Available() = %v, want %v", rb.Available(), len(original))
	}

	// Peek из пустого буфера
	rb.ReadAll()
	n, err = rb.Peek(result)
	if err != ErrBufferEmpty {
		t.Errorf("Peek() error = %v, want ErrBufferEmpty", err)
	}
}

func TestRingBuffer_Skip(t *testing.T) {
	rb, err := NewRingBuffer(10)
	if err != nil {
		t.Fatalf("NewRingBuffer() error = %v", err)
	}

	original := []byte("hello")
	_, err = rb.Write(original)
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	// Skip 2 байта
	err = rb.Skip(2)
	if err != nil {
		t.Errorf("Skip() error = %v", err)
	}
	if rb.Available() != 3 {
		t.Errorf("Available() = %v, want 3", rb.Available())
	}

	// Проверяем, что пропустили правильные байты
	result := make([]byte, 3)
	_, err = rb.Read(result)
	if err != nil {
		t.Errorf("Read() error = %v", err)
	}
	expected := []byte("llo")
	if !bytes.Equal(result, expected) {
		t.Errorf("Read() = %q, want %q", result, expected)
	}

	// Skip больше чем доступно
	err = rb.Skip(10)
	if err != ErrBufferEmpty {
		t.Errorf("Skip() error = %v, want ErrBufferEmpty", err)
	}

	// Skip с отрицательным числом
	err = rb.Skip(-1)
	if err != ErrInvalidSize {
		t.Errorf("Skip() error = %v, want ErrInvalidSize", err)
	}
}

func TestRingBuffer_Wraparound(t *testing.T) {
	rb, err := NewRingBuffer(5)
	if err != nil {
		t.Fatalf("NewRingBuffer() error = %v", err)
	}

	// Запись и чтение для создания wraparound
	_, err = rb.Write([]byte("abc"))
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	buf := make([]byte, 2)
	_, err = rb.Read(buf)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}

	// Теперь запишем данные, которые обернутся вокруг буфера
	_, err = rb.Write([]byte("def"))
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	// Читаем все данные
	expected := []byte("cdef")
	result := make([]byte, 4)
	n, err := rb.Read(result)
	if err != nil {
		t.Errorf("Read() error = %v", err)
	}
	if n != 4 {
		t.Errorf("Read() = %v, want 4", n)
	}
	if !bytes.Equal(result, expected) {
		t.Errorf("Read() = %q, want %q", result, expected)
	}
}

func TestRingBuffer_Reset(t *testing.T) {
	rb, err := NewRingBuffer(10)
	if err != nil {
		t.Fatalf("NewRingBuffer() error = %v", err)
	}

	_, err = rb.Write([]byte("hello"))
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	rb.Reset()

	if rb.Available() != 0 {
		t.Errorf("Available() = %v, want 0", rb.Available())
	}
	if rb.FreeSpace() != rb.Capacity() {
		t.Errorf("FreeSpace() = %v, want %v", rb.FreeSpace(), rb.Capacity())
	}
	if !rb.IsEmpty() {
		t.Errorf("IsEmpty() = false, want true")
	}
}

func TestRingBuffer_IsEmpty_IsFull(t *testing.T) {
	rb, err := NewRingBuffer(5)
	if err != nil {
		t.Fatalf("NewRingBuffer() error = %v", err)
	}

	if !rb.IsEmpty() {
		t.Errorf("IsEmpty() = false, want true")
	}
	if rb.IsFull() {
		t.Errorf("IsFull() = true, want false")
	}

	_, err = rb.Write([]byte("12345"))
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	if rb.IsEmpty() {
		t.Errorf("IsEmpty() = true, want false")
	}
	if !rb.IsFull() {
		t.Errorf("IsFull() = false, want true")
	}
}

func TestRingBuffer_Concurrent(t *testing.T) {
	rb, err := NewRingBuffer(1000)
	if err != nil {
		t.Fatalf("NewRingBuffer() error = %v", err)
	}

	var wg sync.WaitGroup
	goroutines := 10
	iterations := 100

	// Запускаем горутины, которые пишут и читают
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			data := []byte{byte(id)}
			buf := make([]byte, 1)

			for j := 0; j < iterations; j++ {
				// Записываем
				rb.Write(data)
				// Читаем
				rb.Read(buf)
			}
		}(i)
	}

	wg.Wait()

	// Проверяем, что буфер не сломался
	testData := []byte("test")
	n, err := rb.Write(testData)
	if err != nil {
		t.Errorf("Write after concurrent access error = %v", err)
	}
	if n != len(testData) {
		t.Errorf("Write after concurrent access = %v, want %v", n, len(testData))
	}
}

func BenchmarkRingBuffer_Write(b *testing.B) {
	rb, _ := NewRingBuffer(1024)
	data := []byte("hello world")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rb.Write(data)
		if rb.IsFull() {
			rb.Reset()
		}
	}
}

func BenchmarkRingBuffer_Read(b *testing.B) {
	rb, _ := NewRingBuffer(1024)
	data := []byte("hello world")
	buf := make([]byte, 11)

	// Заполняем буфер
	for i := 0; i < 90; i++ {
		rb.Write(data)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rb.Read(buf)
		if rb.IsEmpty() {
			for j := 0; j < 90; j++ {
				rb.Write(data)
			}
		}
	}
}
