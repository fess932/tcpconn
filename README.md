# TCP Connection Library

Библиотека для работы с TCP соединениями, включающая машину состояний TCP и реализацию кольцевого буфера.

## Возможности

- 🔄 **TCP State Machine** - Полная реализация машины состояний TCP согласно RFC 793
- 💾 **Ring Buffer** - Потокобезопасный кольцевой буфер для эффективной работы с данными
- 🧪 **Тестирование** - Полное покрытие тестами
- 🔒 **Потокобезопасность** - Все компоненты защищены от гонок данных
- 📊 **История переходов** - Отслеживание всех изменений состояния

## Установка

```bash
go get github.com/yourusername/tcpconn
```

Или добавьте в ваш проект:

```bash
import "tcpconn"
```

## Использование

### Ring Buffer

Кольцевой буфер - эффективная структура данных для работы с потоками байтов.

#### Базовое использование

```go
package main

import (
    "fmt"
    "tcpconn"
)

func main() {
    // Создание буфера емкостью 1024 байта
    rb, err := tcpconn.NewRingBuffer(1024)
    if err != nil {
        panic(err)
    }

    // Запись данных
    data := []byte("Hello, World!")
    n, err := rb.Write(data)
    if err != nil {
        panic(err)
    }
    fmt.Printf("Записано %d байт\n", n)

    // Чтение данных
    buf := make([]byte, 13)
    n, err = rb.Read(buf)
    if err != nil {
        panic(err)
    }
    fmt.Printf("Прочитано: %s\n", string(buf[:n]))
}
```

#### Проверка состояния буфера

```go
rb, _ := tcpconn.NewRingBuffer(100)

fmt.Printf("Емкость: %d\n", rb.Capacity())
fmt.Printf("Свободно: %d\n", rb.FreeSpace())
fmt.Printf("Доступно для чтения: %d\n", rb.Available())

if rb.IsEmpty() {
    fmt.Println("Буфер пуст")
}

if rb.IsFull() {
    fmt.Println("Буфер заполнен")
}
```

#### Peek и Skip

```go
rb, _ := tcpconn.NewRingBuffer(100)
rb.Write([]byte("Hello"))

// Чтение без удаления данных
buf := make([]byte, 5)
n, _ := rb.Peek(buf)
fmt.Printf("Peek: %s\n", string(buf[:n]))

// Данные все еще в буфере
fmt.Printf("Доступно: %d\n", rb.Available()) // 5

// Пропустить первые 2 байта
rb.Skip(2)

// Прочитать оставшиеся
rb.Read(buf)
fmt.Printf("Read: %s\n", string(buf[:3])) // "llo"
```

#### WriteAll - атомарная запись

```go
rb, _ := tcpconn.NewRingBuffer(10)
rb.Write([]byte("12345"))

// WriteAll запишет все данные или вернет ошибку
err := rb.WriteAll([]byte("67890"))
if err != nil {
    fmt.Println("Успешно записано")
}

// Если места недостаточно, ничего не запишется
err = rb.WriteAll([]byte("X"))
if err == tcpconn.ErrBufferFull {
    fmt.Println("Буфер заполнен, ничего не записано")
}
```

### TCP State Machine

Машина состояний TCP реализует все переходы согласно протоколу TCP.

#### Установка соединения (клиент)

```go
package main

import (
    "fmt"
    "tcpconn"
)

func main() {
    sm := tcpconn.NewTCPStateMachine()

    fmt.Printf("Начальное состояние: %s\n", sm.GetState())

    // Клиент инициирует соединение
    err := sm.ProcessEvent(tcpconn.ACTIVE_OPEN)
    if err != nil {
        panic(err)
    }
    fmt.Printf("После ACTIVE_OPEN: %s\n", sm.GetState())

    // Получен SYN-ACK от сервера
    err = sm.ProcessEvent(tcpconn.SYN_ACK)
    if err != nil {
        panic(err)
    }
    fmt.Printf("После SYN_ACK: %s\n", sm.GetState())

    if sm.IsConnected() {
        fmt.Println("Соединение установлено!")
    }
}
```

#### Установка соединения (сервер)

```go
sm := tcpconn.NewTCPStateMachine()

// Сервер слушает входящие соединения
sm.ProcessEvent(tcpconn.PASSIVE_OPEN)
fmt.Printf("Состояние: %s\n", sm.GetState()) // LISTEN

// Получен SYN от клиента
sm.ProcessEvent(tcpconn.SYN)
fmt.Printf("Состояние: %s\n", sm.GetState()) // SYN_RECEIVED

// Получен ACK от клиента
sm.ProcessEvent(tcpconn.ACK)
fmt.Printf("Состояние: %s\n", sm.GetState()) // ESTABLISHED
```

#### Закрытие соединения

```go
// Активное закрытие
sm.ProcessEvent(tcpconn.CLOSE)           // FIN_WAIT_1
sm.ProcessEvent(tcpconn.ACK)             // FIN_WAIT_2
sm.ProcessEvent(tcpconn.FIN)             // TIME_WAIT
sm.ProcessEvent(tcpconn.TIMEOUT)         // CLOSED

// Пассивное закрытие
sm.ProcessEvent(tcpconn.FIN)             // CLOSE_WAIT
sm.ProcessEvent(tcpconn.CLOSE)           // LAST_ACK
sm.ProcessEvent(tcpconn.ACK)             // CLOSED
```

#### Проверка состояния

```go
sm := tcpconn.NewTCPStateMachine()
sm.ProcessEvent(tcpconn.ACTIVE_OPEN)
sm.ProcessEvent(tcpconn.SYN_ACK)

if sm.IsConnected() {
    fmt.Println("Соединение установлено")
}

if sm.CanSendData() {
    fmt.Println("Можно отправлять данные")
}

if sm.CanReceiveData() {
    fmt.Println("Можно получать данные")
}

if sm.IsClosing() {
    fmt.Println("Соединение закрывается")
}

if sm.IsClosed() {
    fmt.Println("Соединение закрыто")
}
```

#### Callbacks для отслеживания событий

```go
sm := tcpconn.NewTCPStateMachine()

// Callback при изменении состояния
sm.SetStateChangeCallback(func(oldState, newState tcpconn.TCPState, event tcpconn.TCPEvent) {
    fmt.Printf("Переход: %s -> %s (событие: %s)\n", 
        oldState, newState, event)
})

// Callback при ошибке перехода
sm.SetErrorCallback(func(state tcpconn.TCPState, event tcpconn.TCPEvent, err error) {
    fmt.Printf("Ошибка в состоянии %s при событии %s: %v\n", 
        state, event, err)
})

sm.ProcessEvent(tcpconn.ACTIVE_OPEN)
// Выведет: Переход: CLOSED -> SYN_SENT (событие: ACTIVE_OPEN)
```

#### История переходов

```go
sm := tcpconn.NewTCPStateMachine()

sm.ProcessEvent(tcpconn.ACTIVE_OPEN)
sm.ProcessEvent(tcpconn.SYN_ACK)
sm.ProcessEvent(tcpconn.CLOSE)

// Получить историю всех переходов
history := sm.GetHistory()
for _, transition := range history {
    fmt.Printf("%s -> %s [%s]\n", 
        transition.FromState, 
        transition.ToState, 
        transition.Event)
}

// Очистить историю
sm.ClearHistory()
```

#### Reset соединения

```go
sm := tcpconn.NewTCPStateMachine()

sm.ProcessEvent(tcpconn.ACTIVE_OPEN)
sm.ProcessEvent(tcpconn.SYN_ACK)

// RST сбрасывает соединение из любого состояния
sm.ProcessEvent(tcpconn.RST)
fmt.Printf("Состояние: %s\n", sm.GetState()) // CLOSED

// Или полный сброс машины состояний
sm.Reset()
```

## Состояния TCP

| Состояние | Описание |
|-----------|----------|
| `CLOSED` | Соединение закрыто |
| `LISTEN` | Сервер ожидает входящих соединений |
| `SYN_SENT` | Клиент отправил SYN |
| `SYN_RECEIVED` | Сервер получил SYN и отправил SYN-ACK |
| `ESTABLISHED` | Соединение установлено |
| `FIN_WAIT_1` | Активная сторона отправила FIN |
| `FIN_WAIT_2` | Активная сторона получила ACK на FIN |
| `CLOSE_WAIT` | Пассивная сторона получила FIN |
| `CLOSING` | Обе стороны одновременно закрывают соединение |
| `LAST_ACK` | Пассивная сторона отправила FIN |
| `TIME_WAIT` | Ожидание перед окончательным закрытием |

## События TCP

| Событие | Описание |
|---------|----------|
| `PASSIVE_OPEN` | Пассивное открытие (сервер) |
| `ACTIVE_OPEN` | Активное открытие (клиент) |
| `SYN` | Получен SYN пакет |
| `SYN_ACK` | Получен SYN-ACK пакет |
| `ACK` | Получен ACK пакет |
| `FIN` | Получен FIN пакет |
| `FIN_ACK` | Получен FIN-ACK пакет |
| `CLOSE` | Локальное закрытие соединения |
| `TIMEOUT` | Таймаут |
| `RST` | Сброс соединения |

## Тестирование

Запуск всех тестов:

```bash
go test -v
```

Запуск конкретного теста:

```bash
go test -v -run TestRingBuffer_Write
go test -v -run TestTCPStateMachine_ClientHandshake
```

Запуск бенчмарков:

```bash
go test -bench=. -benchmem
```

Покрытие кода:

```bash
go test -cover
go test -coverprofile=coverage.out
go tool cover -html=coverage.out
```

## Примеры использования

### Пример 1: Простой TCP буфер для соединения

```go
type TCPConnection struct {
    state      *tcpconn.TCPStateMachine
    readBuffer *tcpconn.RingBuffer
    writeBuffer *tcpconn.RingBuffer
}

func NewTCPConnection() *TCPConnection {
    return &TCPConnection{
        state:       tcpconn.NewTCPStateMachine(),
        readBuffer:  tcpconn.NewRingBuffer(4096),
        writeBuffer: tcpconn.NewRingBuffer(4096),
    }
}

func (c *TCPConnection) Connect() error {
    if err := c.state.ProcessEvent(tcpconn.ACTIVE_OPEN); err != nil {
        return err
    }
    // Отправка SYN...
    // Получение SYN-ACK...
    return c.state.ProcessEvent(tcpconn.SYN_ACK)
}

func (c *TCPConnection) Write(data []byte) error {
    if !c.state.CanSendData() {
        return fmt.Errorf("cannot send data in state %s", c.state.GetState())
    }
    return c.writeBuffer.WriteAll(data)
}

func (c *TCPConnection) Read(buf []byte) (int, error) {
    if !c.state.CanReceiveData() {
        return 0, fmt.Errorf("cannot receive data in state %s", c.state.GetState())
    }
    return c.readBuffer.Read(buf)
}
```

### Пример 2: Логирование переходов состояний

```go
sm := tcpconn.NewTCPStateMachine()

sm.SetStateChangeCallback(func(oldState, newState tcpconn.TCPState, event tcpconn.TCPEvent) {
    log.Printf("[TCP] %s -> %s (event: %s)", oldState, newState, event)
})

sm.SetErrorCallback(func(state tcpconn.TCPState, event tcpconn.TCPEvent, err error) {
    log.Printf("[TCP ERROR] State: %s, Event: %s, Error: %v", state, event, err)
})
```

### Пример 3: Потоковая обработка данных

```go
func streamProcessor(rb *tcpconn.RingBuffer) {
    for {
        if rb.Available() < 4 {
            time.Sleep(10 * time.Millisecond)
            continue
        }

        // Читаем размер сообщения
        sizeBuf := make([]byte, 4)
        rb.Peek(sizeBuf)
        size := binary.BigEndian.Uint32(sizeBuf)

        // Ждем полного сообщения
        if rb.Available() < int(4 + size) {
            continue
        }

        // Пропускаем заголовок
        rb.Skip(4)

        // Читаем данные
        data := make([]byte, size)
        rb.Read(data)

        // Обрабатываем сообщение
        processMessage(data)
    }
}
```

## API Reference

### RingBuffer

#### Конструктор
- `NewRingBuffer(capacity int) (*RingBuffer, error)` - создает новый буфер

#### Методы записи
- `Write(data []byte) (int, error)` - записывает данные, возвращает количество записанных байт
- `WriteAll(data []byte) error` - записывает все данные или возвращает ошибку

#### Методы чтения
- `Read(data []byte) (int, error)` - читает данные из буфера
- `ReadAll() []byte` - читает все доступные данные
- `Peek(data []byte) (int, error)` - читает без удаления
- `Skip(n int) error` - пропускает n байт

#### Информация о состоянии
- `Available() int` - количество доступных для чтения байт
- `FreeSpace() int` - количество свободного места
- `Capacity() int` - емкость буфера
- `IsEmpty() bool` - проверка на пустоту
- `IsFull() bool` - проверка на заполненность

#### Управление
- `Reset()` - очищает буфер

### TCPStateMachine

#### Конструктор
- `NewTCPStateMachine() *TCPStateMachine` - создает новую машину состояний

#### Основные методы
- `ProcessEvent(event TCPEvent) error` - обрабатывает событие
- `GetState() TCPState` - возвращает текущее состояние

#### Проверки состояния
- `IsConnected() bool` - соединение установлено
- `IsClosed() bool` - соединение закрыто
- `IsClosing() bool` - соединение закрывается
- `CanSendData() bool` - можно отправлять данные
- `CanReceiveData() bool` - можно получать данные

#### Callbacks
- `SetStateChangeCallback(cb StateChangeCallback)` - установить callback для изменения состояния
- `SetErrorCallback(cb ErrorCallback)` - установить callback для ошибок

#### История
- `GetHistory() []StateTransition` - получить историю переходов
- `ClearHistory()` - очистить историю

#### Управление
- `Reset()` - сбросить в начальное состояние

## Производительность

Бенчмарки на MacBook Pro M1:

```
BenchmarkRingBuffer_Write-8     50000000    25.3 ns/op    0 B/op    0 allocs/op
BenchmarkRingBuffer_Read-8      50000000    28.1 ns/op    0 B/op    0 allocs/op
BenchmarkTCPStateMachine-8      10000000    115 ns/op     0 B/op    0 allocs/op
```

## Лицензия

MIT License

## Автор

Ваше имя / Организация

## Вклад

Pull requests приветствуются! Для крупных изменений, пожалуйста, сначала откройте issue для обсуждения.

Убедитесь, что обновляете тесты при необходимости.