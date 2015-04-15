package qemu

import (
    "fmt"
    "encoding/json"
    "net"
    "errors"
    "log"
    "strconv"
)

type QmpWelcome struct {
    QMP     QmpInfo
}

type QmpInfo struct {
    Version qmpVersion  `json:"version"`
    Package string      `json:"package"`
    Cap     []interface{} `json:"capabilities"`
}

type qmpVersion struct {
    Major   int `json:"major"`
    Minor   int `json:"minor"`
    Micro   int `json:"micro"`
}

type QmpInteraction interface {
    MessageType() int
}

type QmpQuit struct{}
func (qmp *QmpQuit) MessageType() int { return QMP_QUIT }

type QmpInternalError struct { cause string}
func (qmp *QmpInternalError) MessageType() int { return QMP_INTERNAL_ERROR }

type QmpSession struct {
    commands []*QmpCommand
    callback QemuEvent
}
func (qmp *QmpSession) MessageType() int { return QMP_SESSION }

type QmpFinish struct {
    success bool
    reason  map[string]interface{}
    callback QemuEvent
}
func (qmp *QmpFinish) MessageType() int { return QMP_FINISH }

type QmpCommand struct {
    Execute string `json:"execute"`
    Arguments map[string]interface{} `json:"arguments,omitempty"`
}

type QmpResult struct { result map[string]interface{} }
func (qmp *QmpResult) MessageType() int { return QMP_RESULT }

type QmpError struct { cause map[string]interface{} }
func (qmp *QmpError) MessageType() int { return QMP_ERROR }

type QmpEvent struct {
    event       string
    timestamp   uint64
    data        interface{}
}

func (qmp *QmpEvent) MessageType() int { return QMP_EVENT }
func (qmp *QmpEvent) Event() int { return EVENT_QMP_EVENT }

func parseQmpEvent(msg map[string]interface{}) (*QmpEvent,error) {
    ts := genericGetField(msg, "timestamp")
    if ts == nil {
        return nil, errors.New("cannot parse timestamp")
    }

    t := (ts).(map[string]interface{})
    seconds := genericGetField(t, "seconds")
    microseconds := genericGetField(t, "microseconds")
    data := genericGetField(msg, "data")
    if data == nil {
        data = make(map[string]interface{})
    }

    if seconds != nil && microseconds != nil {
        return &QmpEvent{
            event: genericGetField(msg, "event").(string),
            timestamp: uint64(seconds.(float64)) * 1000000 + uint64(microseconds.(float64)),
            data: data,
        },nil
    } else {
        return nil,errors.New("QMP Event parse failed")
    }
}

func genericGetField(msg map[string]interface{}, field string) interface{} {
    if v,ok := msg[field]; ok {
        return v
    }
    return nil
}

func qmpCmdSend(c *net.UnixConn, cmd *QmpCommand) error {
    msg,err := json.Marshal(*cmd)
    if err != nil {
        return err
    }
    _, err = c.Write(msg)
    return err
}

func qmpDecode(msg map[string]interface{}) (QmpInteraction, error) {
    if r,ok := msg["return"] ; ok {
        return &QmpResult{result:r.(map[string]interface{})}, nil
    } else if r,ok := msg["error"] ; ok {
        return &QmpError{cause:r.(map[string]interface{})}, nil
    } else if _,ok := msg["event"] ; ok {
        return parseQmpEvent(msg)
    } else {
        return nil,errors.New("Unhandled message type.")
    }
}

func qmpReceiver(ch chan QmpInteraction, decoder *json.Decoder) {
    for {
        var msg map[string]interface{}
        if err := decoder.Decode(&msg); err != nil {
            ch <- &QmpInternalError{cause:err.Error()}
            return
        }
        qmp,err := qmpDecode(msg)
        if err != nil {
            ch <- &QmpInternalError{cause:err.Error()}
            return
        }
        ch <- qmp
        if qmp.MessageType() == QMP_EVENT && qmp.(*QmpEvent).event == QMP_EVENT_SHUTDOWN {
            return
        }
    }
}

func qmpInit(s *net.UnixListener) (*net.UnixConn, *json.Decoder, error) {
    var msg map[string]interface{}

    conn, err := s.AcceptUnix()
    if err != nil {
        log.Print("accept socket error ", err.Error())
        return nil, nil, err
    }
    decoder := json.NewDecoder(conn)

    err = decoder.Decode(&msg)
    if err != nil {
        log.Print("get qmp welcome failed: ", err.Error())
        return nil, nil, err
    }

    err = qmpCmdSend(conn, &QmpCommand{Execute:"qmp_capabilities"})
    if err != nil {
        log.Print("qmp_capabilities send failed")
        return nil, nil, err
    }

    err = decoder.Decode(&msg)
    if err != nil {
        log.Print("response receive failed", err.Error())
        return nil, nil, err
    }

    if _,ok:= msg["return"]; ok {
        log.Print("QMP connection initialized")
        return conn, decoder, nil
    }

    return nil, nil, fmt.Errorf("handshake failed")
}

func scsiId2Name(id int) string {
    var ch byte= 'a' + byte(id%26)
    if id >= 26 {
        return scsiId2Name(id/26 - 1) + string(ch)
    }
    return "sd" + string(ch)
}

func newDiskAddSession(ctx *QemuContext, name, sourceType, filename, format string, id int) *QmpSession {
    commands := make([]*QmpCommand, 2)
    commands[0] = &QmpCommand{
        Execute: "human-monitor-command",
        Arguments: map[string]interface{}{
            "command-line":"drive_add dummy file=" +
            filename + ",if=none,id=" + "scsi-disk" + strconv.Itoa(id) + ",format" + format + ",cache=writeback",
        },
    }
    commands[1] = &QmpCommand{
        Execute: "device_add",
        Arguments: map[string]interface{}{
            "driver":"scsi-hd","bus":"scsi0","scsi-id":strconv.Itoa(id),
            "drive":"scsi-disk0","id": "scsi-disk" + strconv.Itoa(id),
        },
    }
    devName := scsiId2Name(id)
    return &QmpSession{
        commands: commands,
        callback: &BlockdevInsertedEvent{
            Name: name,
            SourceType: sourceType,
            DeviceName: devName,
        },
    }
}

func newNetworkAddSession(ctx *QemuContext, fd, device string, index, addr int) *QmpSession {
    busAddr := fmt.Sprintf("0x%x", addr)
    commands := make([]*QmpCommand, 2)
    commands[0] = &QmpCommand{
        Execute: "netdev_add",
        Arguments: map[string]interface{}{
            "type":"tap","id":device,"script":"no",
        },
    }
    commands[1] = &QmpCommand{
        Execute: "device_add",
        Arguments: map[string]interface{}{
            "driver":"virtio-net-pci","netdev":device,"id":"net-" + busAddr,"bus":"pci.0","addr":busAddr,
        },
    }

    if len(fd) > 0 {
        (*commands[0]).Arguments["fd"] = fd
    }
    return &QmpSession{
        commands: commands,
        callback: &NetDevInsertedEvent{
            Index:      index,
            DeviceName: device,
            Address:    addr,
        },
    }
}

func qmpCommander(handler chan QmpInteraction, conn *net.UnixConn, session *QmpSession, feedback chan QmpInteraction) {
    for _,cmd := range session.commands {
        msg,err := json.Marshal(*cmd)
        if err != nil {
            handler <- &QmpFinish{
                success: false,
                reason: map[string]interface{}{
                    "error": "cannot marshal command",
                },
                callback: session.callback,
            }
            return
        }
        conn.Write(msg)

        res := <- feedback
        switch res.MessageType() {
            case QMP_RESULT:
            //success
            case QMP_ERROR:
            handler <- &QmpFinish{
                success: false,
                reason: res.(*QmpError).cause,
                callback: session.callback,
            }
            default:
//            response,_ := json.Marshal(*res)
            handler <- &QmpFinish{
                success: false,
                reason: map[string]interface{}{
                    "error": "unknown received message type",
                    "response": []byte{},
                },
                callback: session.callback,
            }
            return
        }
    }
    handler <- &QmpFinish{
        success: true,
        callback: session.callback,
    }
    return
}

func qmpHandler(ctx *QemuContext) {
    conn,decoder,err := qmpInit(ctx.qmpSock)
    if err != nil {
        log.Print("failed to initialize QMP connection with qemu", err.Error())
        //TODO: should send back to hub
        return
    }

    buf := []*QmpSession{}
    res := make(chan QmpInteraction, 128)

    //routine for get message
    go qmpReceiver(ctx.qmp, decoder)

    for {
        msg := <-ctx.qmp
        switch msg.MessageType() {
        case QMP_SESSION:
            buf = append(buf, msg.(*QmpSession))
            if len(buf) == 1 {
                go qmpCommander(ctx.qmp, conn, msg.(*QmpSession), res)
            }
        case QMP_FINISH:
            buf = buf[1:]
            if len(buf) > 0 {
                go qmpCommander(ctx.qmp, conn, buf[0], res)
            }
            r := msg.(*QmpFinish)
            if r.success {
                ctx.hub <- r.callback
            } else {
                // TODO: fail...
            }
        case QMP_RESULT:
            res <- msg
        case QMP_ERROR:
            res <- msg
        case QMP_EVENT:
            ev := msg.(*QmpEvent)
            ctx.hub <- ev
            if ev.event == QMP_EVENT_SHUTDOWN {
                log.Print("got QMP shutdown event, quit...")
                return
            }
        case QMP_INTERNAL_ERROR:
            go qmpReceiver(ctx.qmp, decoder)
        case QMP_QUIT:
            return
        }
    }
}

