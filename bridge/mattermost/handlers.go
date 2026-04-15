package bmattermost

import (
	"context"
	"runtime/debug"
	"time"

	"github.com/42wim/matterbridge/bridge/config"
	"github.com/42wim/matterbridge/bridge/helper"
	"github.com/matterbridge/matterclient"
	"github.com/mattermost/mattermost/server/public/model"
)

// handleDownloadAvatar downloads the avatar of userid from channel
// sends a EVENT_AVATAR_DOWNLOAD message to the gateway if successful.
// logs an error message if it fails
func (b *Bmattermost) handleDownloadAvatar(userid string, channel string) {
	rmsg := config.Message{
		Username: "system",
		Text:     "avatar",
		Channel:  channel,
		Account:  b.Account,
		UserID:   userid,
		Event:    config.EventAvatarDownload,
		Extra:    make(map[string][]interface{}),
	}
	if _, ok := b.avatarMap[userid]; !ok {
		var (
			data []byte
			err  error
		)
		data, _, err = b.mc.Client.GetProfileImage(context.TODO(), userid, "")
		if err != nil {
			b.Log.Errorf("ProfileImage download failed for %#v %s", userid, err)
			return
		}

		err = helper.HandleDownloadSize(b.Log, &rmsg, userid+".png", int64(len(data)), b.General)
		if err != nil {
			b.Log.Error(err)
			return
		}
		helper.HandleDownloadData(b.Log, &rmsg, userid+".png", rmsg.Text, "", &data, b.General)
		b.Remote <- rmsg
	}
}

//nolint:wrapcheck
func (b *Bmattermost) handleDownloadFile(rmsg *config.Message, id string) error {
	url, _, _ := b.mc.Client.GetFileLink(context.TODO(), id)
	finfo, _, err := b.mc.Client.GetFileInfo(context.TODO(), id)
	if err != nil {
		return err
	}
	err = helper.HandleDownloadSize(b.Log, rmsg, finfo.Name, finfo.Size, b.General)
	if err != nil {
		return err
	}
	data, _, err := b.mc.Client.DownloadFile(context.TODO(), id, true)
	if err != nil {
		return err
	}
	helper.HandleDownloadData(b.Log, rmsg, finfo.Name, rmsg.Text, url, &data, b.General)
	return nil
}

func (b *Bmattermost) handleMatter() {
	messages := make(chan *config.Message)
	if b.GetString("WebhookBindAddress") != "" {
		b.Log.Debugf("Choosing webhooks based receiving")
		go b.handleMatterHook(messages)
	} else {
		if b.GetString("Token") != "" {
			b.Log.Debugf("Choosing token based receiving")
		} else {
			b.Log.Debugf("Choosing login/password based receiving")
		}
		// if for some reason we only want to sent stuff to mattermost but not receive, return
		if b.GetString("WebhookBindAddress") == "" && b.GetString("WebhookURL") != "" && b.GetString("Token") == "" && b.GetString("Login") == "" {
			b.Log.Debugf("No WebhookBindAddress specified, only WebhookURL. You will not receive messages from mattermost, only sending is possible.")
		}
		go b.superviseHandleMatterClient(messages)
	}
	var ok bool
	for message := range messages {
		message.Avatar = helper.GetAvatar(b.avatarMap, message.UserID, b.General)
		message.Account = b.Account
		message.Text, ok = b.replaceAction(message.Text)
		if ok {
			message.Event = config.EventUserAction
		}
		b.Log.Infof("MM-SEND-GW account=%s channel=%s user=%s text_len=%d", b.Account, message.Channel, message.Username, len(message.Text))
		b.Log.Debugf("<= Sending message from %s on %s to gateway", message.Username, b.Account)
		b.Log.Debugf("<= Message is %#v", message)
		b.Remote <- *message
	}
}

// superviseHandleMatterClient restarts handleMatterClient if it panics.
// Without this, a panic would leave b.mc.MessageChan undrained, which in turn
// would block the matterclient WsReceiver on its send and silently freeze the
// whole bridge.
func (b *Bmattermost) superviseHandleMatterClient(messages chan *config.Message) {
	for attempt := 1; ; attempt++ {
		done := make(chan struct{})
		go func() {
			defer close(done)
			defer func() {
				if r := recover(); r != nil {
					b.Log.Errorf("PANIC in handleMatterClient (attempt %d) account=%s: %v\n%s", attempt, b.Account, r, debug.Stack())
				}
			}()
			b.handleMatterClient(messages)
		}()
		<-done
		b.Log.Warnf("handleMatterClient exited for account=%s — restarting in 2s (attempt %d)", b.Account, attempt)
		time.Sleep(2 * time.Second)
	}
}

//nolint:cyclop
func (b *Bmattermost) handleMatterClient(messages chan *config.Message) {
	b.Log.Infof("handleMatterClient: started for account=%s teamID=%s", b.Account, b.TeamID)

	for message := range b.mc.MessageChan {
		b.Log.Infof("MM-RECV account=%s event=%s team=%s channel=%s type=%q user=%s text_len=%d",
			b.Account, message.Raw.EventType(), message.Team, message.Channel, message.Type, message.Username, len(message.Text))
		b.Log.Debugf("MM-RECV raw data: %#v", message.Raw.GetData())

		if b.skipMessage(message) {
			b.Log.Infof("MM-SKIP account=%s event=%s reason=skipMessage", b.Account, message.Raw.EventType())
			b.Log.Debugf("Skipped message: %#v", message)
			continue
		}
		b.Log.Infof("MM-PASS account=%s event=%s channel=%s — forwarding to messages chan", b.Account, message.Raw.EventType(), message.Channel)

	channelName := b.getChannelName(message.Post.ChannelId)
	if channelName == "" {
		channelName = message.Channel
	}

	// If A message is of type "D" (a private message) from another user
	// Then mark the message as a private message
	if ct, ok := message.Raw.GetData()["channel_type"].(string); ok && ct == "D" {
		channelName = "@private"
	}

		// only download avatars if we have a place to upload them (configured mediaserver)
		if b.General.MediaServerUpload != "" || b.General.MediaDownloadPath != "" {
			b.handleDownloadAvatar(message.UserID, channelName)
		}

		b.Log.Debugf("== Receiving event %#v", message)

		rmsg := &config.Message{
			Username: message.Username,
			UserID:   message.UserID,
			Channel:  channelName,
			Text:     message.Text,
			ID:       message.Post.Id,
			ParentID: message.Post.RootId, // ParentID is obsolete with mattermost
			Extra:    make(map[string][]interface{}),
		}

		// handle mattermost post properties (override username and attachments)
		b.handleProps(rmsg, message)

		// create a text for bridges that don't support native editing
		if message.Raw.EventType() == model.WebsocketEventPostEdited && !b.GetBool("EditDisable") {
			rmsg.Text = message.Text + b.GetString("EditSuffix")
		}

		if message.Raw.EventType() == model.WebsocketEventPostDeleted {
			rmsg.Event = config.EventMsgDelete
		}

		for _, id := range message.Post.FileIds {
			err := b.handleDownloadFile(rmsg, id)
			if err != nil {
				b.Log.Errorf("download failed: %s", err)
			}
		}

		// Use nickname instead of username if defined
		if !b.GetBool("useusername") {
			if nick := b.mc.GetNickName(rmsg.UserID); nick != "" {
				rmsg.Username = nick
			}
		}

		messages <- rmsg
	}
}

func (b *Bmattermost) handleMatterHook(messages chan *config.Message) {
	for {
		message := b.mh.Receive()
		b.Log.Debugf("Receiving from matterhook %#v", message)

		messages <- &config.Message{
			UserID:   message.UserID,
			Username: message.UserName,
			Text:     message.Text,
			Channel:  message.ChannelName,
		}
	}
}

func (b *Bmattermost) handleUploadFile(msg *config.Message) (string, error) {
	var err error
	var res, id string
	channelID := b.getChannelID(msg.Channel)
	for _, f := range msg.Extra["file"] {
		fi := f.(config.FileInfo)
		id, err = b.mc.UploadFile(*fi.Data, channelID, fi.Name)
		if err != nil {
			return "", err
		}
		msg.Text = fi.Comment
		if b.GetBool("PrefixMessagesWithNick") {
			msg.Text = msg.Username + msg.Text
		}
		res, err = b.mc.PostMessageWithFiles(channelID, msg.Text, msg.ParentID, []string{id})
	}
	return res, err
}

//nolint:forcetypeassert
func (b *Bmattermost) handleProps(rmsg *config.Message, message *matterclient.Message) {
	props := message.Post.Props
	if props == nil {
		return
	}
	if _, ok := props["override_username"].(string); ok {
		rmsg.Username = props["override_username"].(string)
	}
	if _, ok := props["attachments"].([]interface{}); ok {
		rmsg.Extra["attachments"] = props["attachments"].([]interface{})
		if rmsg.Text != "" {
			return
		}

		for _, attachment := range rmsg.Extra["attachments"] {
			attach := attachment.(map[string]interface{})
			if attach["text"].(string) != "" {
				rmsg.Text += attach["text"].(string)
				continue
			}
			if attach["fallback"].(string) != "" {
				rmsg.Text += attach["fallback"].(string)
			}
		}
	}
}
