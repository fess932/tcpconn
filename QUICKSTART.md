# Быстрый старт

Краткое руководство по использованию библиотеки tcpconn.

## Установка

```bash
go get tcpconn
```

## Ring Buffer - Кольцевой буфер

### Основное использование

```go
package main

import (
    "fmt"
    "tcpconn"
)

func main() {
    // Создать буфер на 1024 байта
    rb, _ := tcpconn.NewRingBuffer(1024)
    
    // Записать данные
    rb.Write([]byte("Hello, World!"))
    
    // Прочитать данные
    buf := make([]byte, 13)
    n, _ := rb.Read(buf)
    fmt.Println(string(buf[:n])) // "Hello, World!"
}
```

### Полезные методы

```go
rb.Available()     // Сколько байт доступно для чтения
rb.FreeSpace()     // Сколько свободного места
rb.IsEmpty()       // Пуст ли буфер
rb.IsFull()        // Заполнен ли буфер
rb.Peek(buf)       // Прочитать без удаления
rb.Skip(n)         // Пропустить n байт
rb.Reset()         // Очистить буфер
```

## TCP State Machine - Машина состояний TCP

### Клиентское соединение

```go
package main

import (
    "fmt"
    "tcpconn"
)

func main() {
    sm := tcpconn.NewTCPStateMachine()
    
    // Инициировать соединение
    sm.ProcessEvent(tcpconn.ACTIVE_OPEN)    // CLOSED -> SYN_SENT
    sm.ProcessEvent(tcpconn.SYN_ACK)        // SYN_SENT -> ESTABLISHED
    
    fmt.Println(sm.IsConnected()) // true
    
    // Закрыть соединение
    sm.ProcessEvent(tcpconn.CLOSE)          // ESTABLISHED -> FIN_WAIT_1
    sm.ProcessEvent(tcpconn.ACK)            // FIN_WAIT_1 -> FIN_WAIT_2
    sm.ProcessEvent(tcpconn.FIN)            // FIN_WAIT_2 -> TIME_WAIT
    sm.ProcessEvent(tcpconn.TIMEOUT)        // TIME_WAIT -> CLOSED
}
```

### Серверное соединение

```go
sm := tcpconn.NewTCPStateMachine()

// Начать слушать
sm.ProcessEvent(tcpconn.PASSIVE_OPEN)    // CLOSED -> LISTEN

// Принять соединение
sm.ProcessEvent(tcpconn.SYN)             // LISTEN -> SYN_RECEIVED
sm.ProcessEvent(tcpconn.ACK)             // SYN_RECEIVED -> ESTABLISHED

fmt.Println(sm.IsConnected()) // true
```

### Проверка состояния

```go
sm.GetState()          // Текущее состояние
sm.IsConnected()       // Соединение установлено?
sm.IsClosed()          // Соединение закрыто?
sm.IsClosing()         // Соединение закрывается?
sm.CanSendData()       // Можно отправлять данные?
sm.CanReceiveData()    // Можно получать данные?
```

### Callbacks

```go
sm.SetStateChangeCallback(func(old, new tcpconn.TCPState, event tcpconn.TCPEvent) {
    fmt.Printf("%s -> %s [%s]\n", old, new, event)
})

sm.SetErrorCallback(func(state tcpconn.TCPState, event tcpconn.TCPEvent, err error) {
    fmt.Printf("Ошибка: %v\n", err)
})
```

### История переходов

```go
history := sm.GetHistory()
for _, t := range history {
    fmt.Printf("%s -> %s [%s]\n", t.FromState, t.ToState, t.Event)
}
```

## Комбинированное использование

### TCP соединение с буферами

```go
package main

import (
    "fmt"
    "tcpconn"
)

type Connection struct {
    state       *tcpconn.TCPStateMachine
    readBuffer  *tcpconn.RingBuffer
    writeBuffer *tcpconn.RingBuffer
}

func NewConnection() *Connection {
    return &Connection{
        state:       tcpconn.NewTCPStateMachine(),
        readBuffer:  tcpconn.NewRingBuffer(4096),
        writeBuffer: tcpconn.NewRingBuffer(4096),
    }
}

func (c *Connection) Connect() error {
    if err := c.state.ProcessEvent(tcpconn.ACTIVE_OPEN); err != nil {
        return err
    }
    return c.state.ProcessEvent(tcpconn.SYN_ACK)
}

func (c *Connection) Send(data []byte) error {
    if !c.state.CanSendData() {
        return fmt.Errorf("cannot send in state %s", c.state.GetState())
    }
    _, err := c.writeBuffer.Write(data)
    return err
}

func (c *Connection) Receive(buf []byte) (int, error) {
    if !c.state.CanReceiveData() {
        return 0, fmt.Errorf("cannot receive in state %s", c.state.GetState())
    }
    return c.readBuffer.Read(buf)
}

func main() {
    conn := NewConnection()
    conn.Connect()
    
    // Отправка
    conn.Send([]byte("Hello!"))
    
    // Получение (предполагая, что данные уже в readBuffer)
    buf := make([]byte, 100)
    n, _ := conn.Receive(buf)
    fmt.Println(string(buf[:n]))
}
```

## Состояния TCP

```
CLOSED          - Соединение закрыто
LISTEN          - Сервер слушает
SYN_SENT        - Клиент отправил SYN
SYN_RECEIVED    - Сервер получил SYN
ESTABLISHED     - Соединение установлено ✓
FIN_WAIT_1      - Начато закрытие
FIN_WAIT_2      - Ждем FIN от удаленной стороны
CLOSE_WAIT      - Удаленная сторона закрыла соединение
CLOSING         - Одновременное закрытие
LAST_ACK        - Ждем финального ACK
TIME_WAIT       - Ожидание перед закрытием
```

## События TCP

```
PASSIVE_OPEN    - Начать слушать (сервер)
ACTIVE_OPEN     - Инициировать соединение (клиент)
SYN             - Получен SYN пакет
SYN_ACK         - Получен SYN-ACK пакет
ACK             - Получен ACK пакет
FIN             - Получен FIN пакет
FIN_ACK         - Получен FIN-ACK пакет
CLOSE           - Локальное закрытие
TIMEOUT         - Таймаут
RST             - Сброс соединения (переход в CLOSED)
```

## Тестирование

```bash
# Запустить все тесты
go test -v

# Запустить конкретный тест
go test -v -run TestRingBuffer_Write

# Бенчмарки
go test -bench=. -benchmem

# Покрытие
go test -cover
```

## Примеры ошибок

```go
// Ring Buffer ошибки
tcpconn.ErrBufferFull       // Буфер заполнен
tcpconn.ErrBufferEmpty      // Буфер пуст
tcpconn.ErrInvalidCapacity  // Неверная емкость
tcpconn.ErrInvalidSize      // Неверный размер

// State Machine ошибки
tcpconn.ErrInvalidTransition // Недопустимый переход состояния
tcpconn.ErrNilStateMachine   // nil машина состояний
```

## Производительность

```
RingBuffer Write:  ~45 ns/op   (0 allocs)
RingBuffer Read:   ~91 ns/op   (0 allocs)
StateMachine:      ~264 ns/op  (4 allocs)
```

## Потокобезопасность

✅ **RingBuffer** - полностью потокобезопасен  
✅ **TCPStateMachine** - полностью потокобезопасен

## Дополнительная информация

- Полная документация: [README.md](README.md)
- Примеры использования: [examples_test.go](examples_test.go)
- Расширенные примеры: [example_usage.go](example_usage.go)

## Лицензия

MIT License