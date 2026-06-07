package tui

// Screen is the top-level TUI mode.
type Screen int

const (
	ScreenDashboard Screen = iota
	ScreenFeatureDetail
	ScreenForm
	ScreenAction
)

// FormKind identifies which form is active.
type FormKind int

const (
	FormNone FormKind = iota
	FormNewFeatureName
	FormFeatureBrief
	FormInitName
	FormInitVision
	FormQuickTask
	FormAsk
	FormActionReply
)

// ActionKind identifies a running workflow action.
type ActionKind int

const (
	ActionNone ActionKind = iota
	ActionSpecify
	ActionDesign
	ActionTasks
	ActionImplement
	ActionRun
	ActionQuickTask
	ActionAsk
)
