// Package chat gives TSX nsgo/ui/chat, ui/chat's conversation components,
// for an app that imports this package for its effect beside tsx/ui.
// ConversationState is a class: new ConversationState() makes one, which
// a transcript shares with its turns and its composer.
package chat

import (
	"github.com/oneo-me/nsgo/core"
	"github.com/oneo-me/nsgo/tsx"
	"github.com/oneo-me/nsgo/ui/chat"

	_ "github.com/oneo-me/nsgo/tsx/ui"
)

const module = "nsgo/ui/chat"

func init() {
	tsx.RegisterClass(module, "ConversationState", tsx.Class[chat.ConversationState]{
		New:     chat.NewConversationState,
		Getters: []string{"AtLatest", "ComposerHeight"},
	})
	tsx.Optional[chat.Activity]("Status", "MaxHeight")
	tsx.Optional[chat.Option]("Image")
	tsx.Optional[chat.Chip]("Image")
	tsx.Register(module, map[string]core.Node{
		"Conversation": chat.Conversation{}, "FloatingConversation": chat.FloatingConversation{}, "GlassPanel": chat.GlassPanel{},
		"Turn": chat.Turn{}, "ActivityRow": chat.ActivityRow{}, "Message": chat.Message{}, "Bubble": chat.Bubble{},
		"Composer": chat.Composer{}, "IconAction": chat.IconAction{}, "AttachmentPreview": chat.AttachmentPreview{},
		"Chip": chat.Chip{}, "ChipPopover": chat.ChipPopover{}, "Picker": chat.Picker{},
		"EffortPicker": chat.EffortPicker{}, "ContextMeter": chat.ContextMeter{}, "Quota": chat.Quota{},
		"QuotaSummary": chat.QuotaSummary{}, "Shimmer": chat.Shimmer{},
	},
		tsx.Enum(map[string]chat.Status{"running": chat.Running, "waiting": chat.Waiting, "completed": chat.Completed,
			"failed": chat.Failed, "cancelled": chat.Cancelled}),
		tsx.Event("OnChange", "id"), tsx.Event("OnDraftChange", "text"), tsx.Event("OnDropFiles", "uris"),
		tsx.Event("OnOpenChange", "open"),
	)
	// What these show as text is not their accessibility label.
	tsx.Register(module, map[string]core.Node{"Attachment": chat.Attachment{}, "ResultSummary": chat.ResultSummary{}},
		tsx.Rename("Label", "label"), tsx.Rename("ActionLabel", "actionLabel"), tsx.Rename("MoreLabel", "moreLabel"),
		tsx.Rename("LessLabel", "lessLabel"), tsx.Event("OnSelect", "id"))
}
