package napcat

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tidwall/gjson"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/driver"

	"miss-raspberry-agent/internal/tools/todo_list"
)

// Message is the struct used for internal communication.
type Message struct {
	UserID      int64
	GroupID     int64
	MessageType string
	NickName    string
	Content     string
	RawEvent    *zero.Event
}

// HistoryMessage is a record of one historical message. Content only contains
// the text payload; non-text messages (e.g. images, voice) leave Content empty.
type HistoryMessage struct {
	MessageID  int64
	MessageSeq int64
	Time       int64
	UserID     int64
	NickName   string
	Content    string
}

// NapcatClient struct.
type NapcatClient struct {
	// Configuration.
	config *NapcatClientConfig

	// todoList is the queue that relevant incoming messages are pushed into; it belongs to the
	// agent and is wired up before Start.
	todoList *todo_list.Store

	// directory is the in-memory reverse index of group members, loaded at startup.
	directory *MemberDirectory

	// Outgoing: messages to send (Agent -> NapCat)
	Outgoing chan Message

	// Control.
	mu      sync.RWMutex
	running bool
	done    chan struct{}
}

// DefaultConfig.
func DefaultConfig() *NapcatClientConfig {
	return &NapcatClientConfig{
		WebSocketURL:             "ws://127.0.0.1:3001",
		AccessToken:              "",
		NickName:                 []string{"bot"},
		CommandPrefix:            "/",
		SuperUsers:               []int64{},
		DirectoryRefreshInterval: defaultDirectoryRefreshInterval,
	}
}

// Constructor.
func NewClient(cfg *NapcatClientConfig) *NapcatClient {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	if cfg.DirectoryRefreshInterval <= 0 {
		cfg.DirectoryRefreshInterval = defaultDirectoryRefreshInterval
	}

	return &NapcatClient{
		config:    cfg,
		directory: NewMemberDirectory(),
		Outgoing:  make(chan Message, 100),
		done:      make(chan struct{}),
	}
}

// SetTodoList configures the todo queue that relevant received messages (private chats and
// group messages mentioning the bot) are pushed into. It must be called before Start so that
// no qualifying message is dropped.
func (c *NapcatClient) SetTodoList(todo *todo_list.Store) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.todoList = todo
}

// Start starts the client (non-blocking).
func (c *NapcatClient) Start() error {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return nil
	}
	c.running = true
	c.mu.Unlock()

	// Register the message handler.
	zero.OnMessage(func(ctx *zero.Ctx) bool {
		// Filtering logic can be added here.
		return true
	}).Handle(func(ctx *zero.Ctx) {
		nickname := ""
		if ctx.Event.Sender != nil {
			nickname = ctx.Event.Sender.NickName
		}
		msg := Message{
			UserID:      ctx.Event.UserID,
			GroupID:     ctx.Event.GroupID,
			MessageType: ctx.Event.MessageType,
			NickName:    nickname,
			Content:     ctx.Event.Message.String(),
			RawEvent:    ctx.Event,
		}

		// Route the message: relevant messages are queued for the agent to process.
		c.HandleIncomingMessage(msg)
	})

	// Start the goroutine that processes Outgoing messages.
	go c.processOutgoing()

	// Start ZeroBot (run in a goroutine to avoid blocking).
	go func() {
		zero.Run(&zero.Config{
			NickName:      c.config.NickName,
			CommandPrefix: c.config.CommandPrefix,
			SuperUsers:    c.config.SuperUsers,
			Driver: []zero.Driver{
				driver.NewWebSocketClient(c.config.WebSocketURL, c.config.AccessToken),
			},
		})
	}()

	// Load and periodically refresh the group member directory once a bot connects.
	go c.groupSyncLoop()

	log.Println("[Napcat] Client started")
	return nil
}

// Stop stops the client.
func (c *NapcatClient) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.running {
		return
	}

	c.running = false
	close(c.done)
	close(c.Outgoing)

	log.Println("[Napcat] Client stopped")
}

// SyncDirectory loads the bot's friend list, group list and each group's member
// list into the in-memory directory. It returns an error when no bot is connected
// or the friend/group list cannot be fetched; a failure for an individual group is
// logged and skipped so the rest of the directory is still usable.
func (c *NapcatClient) SyncDirectory(ctx context.Context) error {
	bot := c.bot()
	if bot == nil {
		return errors.New("napcat: no bot connected")
	}

	friendRsp := bot.CallActionWithContext(ctx, "get_friend_list", zero.Params{})
	if friendRsp.RetCode != 0 {
		return fmt.Errorf("napcat: get_friend_list failed (retcode=%d, message=%s, wording=%s)", friendRsp.RetCode, friendRsp.Message, friendRsp.Wording)
	}
	friends := ParseFriendList(friendRsp.Data)

	rsp := bot.CallActionWithContext(ctx, "get_group_list", zero.Params{})
	if rsp.RetCode != 0 {
		return fmt.Errorf("napcat: get_group_list failed (retcode=%d, message=%s, wording=%s)", rsp.RetCode, rsp.Message, rsp.Wording)
	}
	groupIDs := ParseGroupList(rsp.Data)

	groupMembers := make(map[int64][]int64, len(groupIDs))
	for _, groupID := range groupIDs {
		memberRsp := bot.CallActionWithContext(ctx, "get_group_member_list", zero.Params{"group_id": groupID})
		if memberRsp.RetCode != 0 {
			log.Printf("[Napcat] get_group_member_list failed for group %d (retcode=%d), skipping", groupID, memberRsp.RetCode)
			continue
		}
		groupMembers[groupID] = ParseGroupMemberList(memberRsp.Data)
	}

	c.directory.Replace(friends, BuildMemberGroups(groupMembers))
	log.Printf("[Napcat] directory loaded: %d friends, %d groups, %d members, %d associations",
		c.directory.FriendCount(), len(groupIDs), c.directory.MemberCount(), c.directory.EdgeCount())
	return nil
}

// bot returns a connected ZeroBot context, or nil when no bot is connected yet.
func (c *NapcatClient) bot() *zero.Ctx {
	var bot *zero.Ctx
	zero.RangeBot(func(_ int64, zb *zero.Ctx) bool {
		bot = zb
		return false
	})
	return bot
}

// callAction invokes a OneBot action and reports whether it succeeded (retcode 0).
func (c *NapcatClient) callAction(ctx context.Context, action string, params zero.Params) bool {
	bot := c.bot()
	if bot == nil {
		log.Printf("[Napcat] no bot connected, %s failed", action)
		return false
	}
	rsp := bot.CallActionWithContext(ctx, action, params)
	if rsp.RetCode != 0 {
		log.Printf("[Napcat] %s failed (retcode=%d, message=%s, wording=%s)", action, rsp.RetCode, rsp.Message, rsp.Wording)
		return false
	}
	return true
}

// TrySendToMemberViaGroups attempts to reach a member through each candidate
// group's temporary session in order, stopping at the first success. The send
// function reports whether the attempt through a given group succeeded.
func TrySendToMemberViaGroups(groups []int64, send func(groupID int64) bool) (int64, bool) {
	for _, groupID := range groups {
		if send(groupID) {
			return groupID, true
		}
	}
	return 0, false
}

// sendPrivate sends content to userID. Friends receive a normal private message;
// non-friends are reached through a group temporary session, trying every group
// the bot shares with the user until one succeeds. As a last resort (and while the
// directory is not yet loaded) it attempts a plain private message.
func (c *NapcatClient) sendPrivate(ctx context.Context, userID int64, content string) bool {
	if c.directory.Loaded() && c.directory.IsFriend(userID) {
		return c.callAction(ctx, "send_private_msg", zero.Params{"user_id": userID, "message": content})
	}

	groupID, ok := TrySendToMemberViaGroups(c.directory.Groups(userID), func(groupID int64) bool {
		return c.callAction(ctx, "send_private_msg", zero.Params{
			"user_id":  userID,
			"group_id": groupID,
			"message":  content,
		})
	})
	if ok {
		log.Printf("[Napcat] temporary session message sent to %d via group %d", userID, groupID)
		return true
	}

	return c.callAction(ctx, "send_private_msg", zero.Params{"user_id": userID, "message": content})
}

// groupSyncLoop keeps the in-memory directory up to date: it retries the initial
// load until it succeeds, then refreshes on the configured interval. It exits when
// the client stops.
func (c *NapcatClient) groupSyncLoop() {
	retryTicker := time.NewTicker(2 * time.Second)
	defer retryTicker.Stop()
	for {
		select {
		case <-c.done:
			return
		case <-retryTicker.C:
		}

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		err := c.SyncDirectory(ctx)
		cancel()
		if err != nil {
			log.Printf("[Napcat] directory sync failed, retrying: %v", err)
			continue
		}
		break
	}

	refreshTicker := time.NewTicker(c.config.DirectoryRefreshInterval)
	defer refreshTicker.Stop()
	for {
		select {
		case <-c.done:
			return
		case <-refreshTicker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			err := c.SyncDirectory(ctx)
			cancel()
			if err != nil {
				log.Printf("[Napcat] periodic directory sync failed: %v", err)
			}
		}
	}
}

// SendMessage sends a private message (called by the Agent).
func (c *NapcatClient) SendMessage(userID int64, content string) bool {
	msg := Message{
		UserID:  userID,
		Content: content,
	}

	if !c.enqueue(msg) {
		return false
	}
	log.Printf("[Napcat] message queued for sending: %s", content)
	return true
}

// SendGroupMessage sends a group message (called by the Agent).
func (c *NapcatClient) SendGroupMessage(groupID int64, content string) bool {
	msg := Message{
		GroupID: groupID,
		Content: content,
	}

	if !c.enqueue(msg) {
		return false
	}
	log.Printf("[Napcat] group message queued for sending: %s", content)
	return true
}

// enqueue puts a message into the Outgoing queue; returns false if the queue is full.
func (c *NapcatClient) enqueue(msg Message) bool {
	select {
	case c.Outgoing <- msg:
		return true
	default:
		log.Println("[Napcat] Outgoing channel is full, send failed")
		return false
	}
}

// GetMessageHistory fetches historical messages for a private or group chat.
//
// When beforeSeq is 0 it returns the latest count messages; when greater than 0
// it returns count messages whose message_seq is less than beforeSeq, for paging
// backwards. The returned messages are ordered as returned by the API.
func (c *NapcatClient) GetMessageHistory(ctx context.Context, targetType string, targetID, beforeSeq int64, count int) ([]HistoryMessage, error) {
	action := "get_group_msg_history"
	params := zero.Params{
		"group_id": targetID,
		"count":    count,
	}
	if targetType == "private" {
		action = "get_friend_msg_history"
		params = zero.Params{
			"user_id": strconv.FormatInt(targetID, 10),
			"count":   count,
		}
	}
	if beforeSeq > 0 {
		if targetType == "private" {
			params["message_seq"] = strconv.FormatInt(beforeSeq, 10)
		} else {
			params["message_seq"] = beforeSeq
			params["message_id"] = beforeSeq
		}
	}

	var rsp zero.APIResponse
	called := false
	zero.RangeBot(func(_ int64, zb *zero.Ctx) bool {
		called = true
		rsp = zb.CallActionWithContext(ctx, action, params)
		return false
	})
	if !called {
		return nil, errors.New("napcat: no bot connected")
	}
	if rsp.RetCode != 0 {
		return nil, fmt.Errorf("napcat: %s failed (retcode=%d, message=%s, wording=%s)", action, rsp.RetCode, rsp.Message, rsp.Wording)
	}

	return ParseHistoryResponse(rsp.Data), nil
}

// ParseHistoryResponse parses the data part of a get_*_msg_history response
// into historical messages. NapCat returns the data as data.messages; this also
// accepts a direct array of messages.
func ParseHistoryResponse(data gjson.Result) []HistoryMessage {
	// Calling Array() directly on data treats the whole object as one "empty
	// message"; the messages field must be read instead.
	messagesResult := data.Get("messages")
	if !messagesResult.Exists() {
		messagesResult = data
	}

	messages := make([]HistoryMessage, 0, len(messagesResult.Array()))
	for _, item := range messagesResult.Array() {
		seq := item.Get("message_seq").Int()
		if seq == 0 {
			seq = item.Get("message_id").Int()
		}
		messages = append(messages, HistoryMessage{
			MessageID:  item.Get("message_id").Int(),
			MessageSeq: seq,
			Time:       item.Get("time").Int(),
			UserID:     item.Get("user_id").Int(),
			NickName:   item.Get("sender.nickname").String(),
			Content:    extractText(item),
		})
	}
	return messages
}

// extractText extracts plain text from a OneBot message.
// message may be a string or an array of message segments; only segments of
// type=text are kept.
func extractText(item gjson.Result) string {
	message := item.Get("message")
	if message.Type == gjson.String {
		return message.String()
	}
	var sb strings.Builder
	for _, seg := range message.Array() {
		if seg.Get("type").String() != "text" {
			continue
		}
		sb.WriteString(seg.Get("data.text").String())
	}
	return sb.String()
}

// processOutgoing is the goroutine that processes Outgoing messages.
func (c *NapcatClient) processOutgoing() {
	for {
		select {
		case <-c.done:
			return
		case msg := <-c.Outgoing:
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

			if msg.GroupID != 0 {
				zero.RangeBot(func(_ int64, zb *zero.Ctx) bool {
					zb.SendGroupMessage(msg.GroupID, msg.Content)
					return false
				})
				log.Printf("[Napcat] group message sent to %d: %s", msg.GroupID, msg.Content)
			} else if c.sendPrivate(ctx, msg.UserID, msg.Content) {
				log.Printf("[Napcat] private message sent to %d: %s", msg.UserID, msg.Content)
			} else {
				log.Printf("[Napcat] private message to %d could not be sent: %s", msg.UserID, msg.Content)
			}

			cancel()
		}
	}
}
