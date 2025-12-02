# Интеграция библиотеки в сторонние проекты

Это руководство описывает различные способы интеграции библиотеки tcpconn в ваши проекты.

## Способ 1: Импорт как Go модуль (рекомендуется)

### Локальный импорт

Если библиотека находится в локальной файловой системе:

```bash
# В корне вашего проекта
go mod edit -replace tcpconn=/path/to/tcpconn
go mod tidy
```

В вашем коде:

```go
package main

import (
    "fmt"
    "tcpconn"
)

func main() {
    rb, _ := tcpconn.NewRingBuffer(1024)
    sm := tcpconn.NewTCPStateMachine()
    // ...
}
```

### Импорт из Git репозитория

```bash
go get github.com/yourusername/tcpconn
```

```go
import "github.com/yourusername/tcpconn"
```

## Способ 2: Копирование файлов

Если вы хотите включить исходный код напрямую в ваш проект.

### Вариант A: Полная копия

Скопируйте все файлы библиотеки:

```bash
# Создайте директорию в вашем проекте
mkdir -p myproject/pkg/tcpconn

# Скопируйте файлы
cp tcpconn/ringbuffer.go myproject/pkg/tcpconn/
cp tcpconn/tcpstate.go myproject/pkg/tcpconn/
cp tcpconn/example_usage.go myproject/pkg/tcpconn/

# Опционально: скопируйте тесты
cp tcpconn/*_test.go myproject/pkg/tcpconn/
```

Измените package name в начале каждого файла:

```go
// Было:
package tcpconn

// Стало:
package tcpconn  // или package tcp, package network и т.д.
```

Использование:

```go
package main

import "myproject/pkg/tcpconn"

func main() {
    rb, _ := tcpconn.NewRingBuffer(1024)
}
```

### Вариант B: Только Ring Buffer

Если вам нужен только кольцевой буфер:

```bash
cp tcpconn/ringbuffer.go myproject/pkg/buffer/
```

Измените package:

```go
package buffer
```

Использование:

```go
import "myproject/pkg/buffer"

rb, _ := buffer.NewRingBuffer(1024)
```

### Вариант C: Только TCP State Machine

Если нужна только машина состояний:

```bash
cp tcpconn/tcpstate.go myproject/pkg/tcp/
```

Измените package:

```go
package tcp
```

Использование:

```go
import "myproject/pkg/tcp"

sm := tcp.NewTCPStateMachine()
```

## Способ 3: Vendor директория

Для проектов, использующих vendoring:

```bash
# В корне вашего проекта
go mod vendor

# Или вручную
mkdir -p vendor/tcpconn
cp tcpconn/*.go vendor/tcpconn/
```

## Примеры интеграции

### Пример 1: Простой TCP клиент

```go
package main

import (
    "fmt"
    "net"
    "tcpconn"
)

type TCPClient struct {
    conn       net.Conn
    state      *tcpconn.TCPStateMachine
    readBuffer *tcpconn.RingBuffer
}

func NewTCPClient(addr string) (*TCPClient, error) {
    conn, err := net.Dial("tcp", addr)
    if err != nil {
        return nil, err
    }

    client := &TCPClient{
        conn:       conn,
        state:      tcpconn.NewTCPStateMachine(),
        readBuffer: tcpconn.MustNewRingBuffer(8192),
    }

    // Симулируем установку соединения
    client.state.ProcessEvent(tcpconn.ACTIVE_OPEN)
    client.state.ProcessEvent(tcpconn.SYN_ACK)

    return client, nil
}

func (c *TCPClient) Send(data []byte) error {
    if !c.state.CanSendData() {
        return fmt.Errorf("cannot send in state %s", c.state.GetState())
    }

    _, err := c.conn.Write(data)
    return err
}

func (c *TCPClient) Receive() ([]byte, error) {
    if !c.state.CanReceiveData() {
        return nil, fmt.Errorf("cannot receive in state %s", c.state.GetState())
    }

    buf := make([]byte, 4096)
    n, err := c.conn.Read(buf)
    if err != nil {
        return nil, err
    }

    // Сохраняем в ring buffer для последующей обработки
    c.readBuffer.Write(buf[:n])
    return buf[:n], nil
}

func (c *TCPClient) Close() error {
    c.state.ProcessEvent(tcpconn.CLOSE)
    return c.conn.Close()
}

func main() {
    client, err := NewTCPClient("localhost:8080")
    if err != nil {
        panic(err)
    }
    defer client.Close()

    // Отправка
    client.Send([]byte("Hello, Server!"))

    // Получение
    data, _ := client.Receive()
    fmt.Printf("Received: %s\n", data)
}
```

### Пример 2: Протокол с кадрированием

```go
package main

import (
    "encoding/binary"
    "io"
    "tcpconn"
)

// FrameProtocol - протокол с фиксированным заголовком
type FrameProtocol struct {
    reader io.Reader
    writer io.Writer
    buffer *tcpconn.RingBuffer
}

func NewFrameProtocol(r io.Reader, w io.Writer) *FrameProtocol {
    return &FrameProtocol{
        reader: r,
        writer: w,
        buffer: tcpconn.MustNewRingBuffer(65536),
    }
}

// SendFrame отправляет кадр с заголовком длины
func (fp *FrameProtocol) SendFrame(data []byte) error {
    // Заголовок: 4 байта длины
    header := make([]byte, 4)
    binary.BigEndian.PutUint32(header, uint32(len(data)))

    // Отправляем заголовок
    if _, err := fp.writer.Write(header); err != nil {
        return err
    }

    // Отправляем данные
    _, err := fp.writer.Write(data)
    return err
}

// ReceiveFrame получает кадр
func (fp *FrameProtocol) ReceiveFrame() ([]byte, error) {
    // Читаем данные в буфер
    tmp := make([]byte, 4096)
    n, err := fp.reader.Read(tmp)
    if err != nil {
        return nil, err
    }
    fp.buffer.Write(tmp[:n])

    // Проверяем заголовок
    if fp.buffer.Available() < 4 {
        return nil, io.ErrShortBuffer
    }

    header := make([]byte, 4)
    fp.buffer.Peek(header)
    length := binary.BigEndian.Uint32(header)

    // Ждем полного кадра
    if fp.buffer.Available() < int(4+length) {
        return nil, io.ErrShortBuffer
    }

    // Читаем кадр
    fp.buffer.Skip(4) // пропускаем заголовок
    data := make([]byte, length)
    fp.buffer.Read(data)

    return data, nil
}
```

### Пример 3: Обертка для существующего net.Conn

```go
package main

import (
    "net"
    "tcpconn"
)

// TrackedConnection - соединение с отслеживанием состояния
type TrackedConnection struct {
    net.Conn
    state   *tcpconn.TCPStateMachine
    rxBuffer *tcpconn.RingBuffer
    txBuffer *tcpconn.RingBuffer
}

func WrapConnection(conn net.Conn) *TrackedConnection {
    tc := &TrackedConnection{
        Conn:     conn,
        state:    tcpconn.NewTCPStateMachine(),
        rxBuffer: tcpconn.MustNewRingBuffer(32768),
        txBuffer: tcpconn.MustNewRingBuffer(32768),
    }

    // Настраиваем callbacks
    tc.state.SetStateChangeCallback(func(old, new tcpconn.TCPState, event tcpconn.TCPEvent) {
        log.Printf("TCP: %s -> %s [%s]", old, new, event)
    })

    // Устанавливаем начальное состояние
    tc.state.ProcessEvent(tcpconn.ACTIVE_OPEN)
    tc.state.ProcessEvent(tcpconn.SYN_ACK)

    return tc
}

func (tc *TrackedConnection) Write(data []byte) (int, error) {
    if !tc.state.CanSendData() {
        return 0, fmt.Errorf("cannot write in state %s", tc.state.GetState())
    }

    // Буферизуем данные
    n, err := tc.txBuffer.Write(data)
    if err != nil {
        return n, err
    }

    // Отправляем
    return tc.Conn.Write(data)
}

func (tc *TrackedConnection) Read(buf []byte) (int, error) {
    if !tc.state.CanReceiveData() {
        return 0, fmt.Errorf("cannot read in state %s", tc.state.GetState())
    }

    return tc.Conn.Read(buf)
}

func (tc *TrackedConnection) GetState() tcpconn.TCPState {
    return tc.state.GetState()
}

func (tc *TrackedConnection) GetHistory() []tcpconn.StateTransition {
    return tc.state.GetHistory()
}
```

## Настройка для разных проектов

### Изменение размера буфера по умолчанию

```go
// Создайте helper функцию
func NewDefaultRingBuffer() (*tcpconn.RingBuffer, error) {
    return tcpconn.NewRingBuffer(16384) // ваш размер по умолчанию
}
```

### Добавление собственных состояний

Если нужны дополнительные состояния, расширьте enum:

```go
const (
    // Стандартные состояния
    tcpconn.CLOSED
    tcpconn.ESTABLISHED
    // ...

    // Ваши дополнительные состояния
    AUTHENTICATING = 100
    AUTHENTICATED  = 101
)
```

### Интеграция с логированием

```go
sm := tcpconn.NewTCPStateMachine()

sm.SetStateChangeCallback(func(old, new tcpconn.TCPState, event tcpconn.TCPEvent) {
    log.WithFields(log.Fields{
        "old_state": old,
        "new_state": new,
        "event":     event,
    }).Info("TCP state changed")
})

sm.SetErrorCallback(func(state tcpconn.TCPState, event tcpconn.TCPEvent, err error) {
    log.WithFields(log.Fields{
        "state": state,
        "event": event,
        "error": err,
    }).Error("TCP state error")
})
```

## Тестирование интеграции

### Проверка импорта

```bash
# Убедитесь, что библиотека доступна
go list -m tcpconn

# Запустите тесты вашего проекта
go test ./...
```

### Тестирование с mock

```go
package myapp_test

import (
    "testing"
    "tcpconn"
)

func TestMyConnection(t *testing.T) {
    // Создаём компоненты для тестирования
    sm := tcpconn.NewTCPStateMachine()
    rb, _ := tcpconn.NewRingBuffer(1024)

    // Ваша логика тестирования
    sm.ProcessEvent(tcpconn.ACTIVE_OPEN)
    if !sm.GetState() == tcpconn.SYN_SENT {
        t.Error("Expected SYN_SENT state")
    }

    // Тестируем буфер
    testData := []byte("test")
    rb.Write(testData)
    
    buf := make([]byte, 4)
    n, _ := rb.Read(buf)
    if n != 4 {
        t.Errorf("Expected 4 bytes, got %d", n)
    }
}
```

## Troubleshooting

### Проблема: Import cycle

**Решение:** Переместите библиотеку в отдельный package или используйте vendor.

### Проблема: Package name conflicts

**Решение:** Используйте alias при импорте:

```go
import (
    tcp "tcpconn"
    mytcp "myproject/tcp"
)
```

### Проблема: Версии Go

Библиотека требует Go 1.18+. Проверьте:

```bash
go version
```

## Best Practices

1. **Используйте defer для закрытия ресурсов:**
   ```go
   sm := tcpconn.NewTCPStateMachine()
   defer sm.Reset()
   ```

2. **Проверяйте ошибки:**
   ```go
   rb, err := tcpconn.NewRingBuffer(1024)
   if err != nil {
       return fmt.Errorf("failed to create buffer: %w", err)
   }
   ```

3. **Используйте callbacks для мониторинга:**
   ```go
   sm.SetStateChangeCallback(yourLogger)
   sm.SetErrorCallback(yourErrorHandler)
   ```

4. **Выбирайте правильный размер буфера:**
   - Для быстрых коннекций: 4-8 KB
   - Для обычных: 16-32 KB
   - Для high-throughput: 64-128 KB

5. **Тестируйте граничные случаи:**
   - Полный буфер
   - Пустой буфер
   - Недопустимые переходы состояний
   - Конкурентный доступ

## Документация

- [README.md](README.md) - Полная документация
- [QUICKSTART.md](QUICKSTART.md) - Быстрый старт
- [PROJECT.md](PROJECT.md) - Структура проекта
- [examples_test.go](examples_test.go) - Примеры кода

## Поддержка

При возникновении проблем:
1. Проверьте документацию
2. Изучите примеры в examples_test.go
3. Запустите тесты: `go test -v`
4. Проверьте совместимость версий Go

## Лицензия

MIT License - вы можете свободно использовать, модифицировать и распространять эту библиотеку.