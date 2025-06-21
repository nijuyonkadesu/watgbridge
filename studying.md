<<<<<<< Updated upstream
# utils handlers
=======
# telegram/handlers.go

>>>>>>> Stashed changes
```go
func AddTelegramHandlers() {
	func(msg *gotgbot.Message) bool {
	func(cq *gotgbot.CallbackQuery) bool {
        } }}


# long running
func BridgeTelegramToWhatsAppHandler(b *gotgbot.Bot, c *ext.Context) error {
func SendToWhatsAppHandler(b *gotgbot.Bot, c *ext.Context) error {

# TODO: check logic
func SyncTopicNamesHandler(b *gotgbot.Bot, c *ext.Context) error {
func UnlinkThreadHandler(b *gotgbot.Bot, c *ext.Context) error {
func SyncContactsHandler(b *gotgbot.Bot, c *ext.Context) error {
func ClearMessageIdPairsHistoryHandler(b *gotgbot.Bot, c *ext.Context) error {

# TOOD: Chat setup
func SetTargetGroupChatHandler(b *gotgbot.Bot, c *ext.Context) error {
func SetTargetPrivateChatHandler(b *gotgbot.Bot, c *ext.Context) error {

---
func GetWhatsAppGroupsHandler(b *gotgbot.Bot, c *ext.Context) error {
func JoinInviteLinkHandler(b *gotgbot.Bot, c *ext.Context) error {


func BlockCommandHandler(b *gotgbot.Bot, c *ext.Context) error {
func UnblockCommandHandler(b *gotgbot.Bot, c *ext.Context) error {
func handleBlockUnblockUser(b *gotgbot.Bot, c *ext.Context, action events.BlocklistChangeAction) error {




# shd be simple
func StartCommandHandler(b *gotgbot.Bot, c *ext.Context) error {
func HelpCommandHandler(b *gotgbot.Bot, c *ext.Context) error {

func FindContactHandler(b *gotgbot.Bot, c *ext.Context) error {
func GetProfilePictureHandler(b *gotgbot.Bot, c *ext.Context) error {

func RevokeCallbackHandler(b *gotgbot.Bot, c *ext.Context) error {
func RevokeCommandHandler(b *gotgbot.Bot, c *ext.Context) error {

func RestartWhatsAppConnectionHandler(b *gotgbot.Bot, c *ext.Context) error {
func UpdateAndRestartHandler(b *gotgbot.Bot, c *ext.Context) error {
```
<<<<<<< Updated upstream
=======

---

# Plan 2025-06-24

1. telegram handlers
2. whatsapp handlers
3. gorm data model
4. telegram utils
5. whatsapp utils

---

# utils/telegram.go

```go
# Main chunk
func TgSendToWhatsApp(b *gotgbot.Bot, c *ext.Context,
func TgDownloadByFilePath(b *gotgbot.Bot, filePath string) ([]byte, error) {

# db
func TgGetOrMakeThreadFromWa(waChatId string, tgChatId int64, threadName string) (int64, error) {

# Basic
func TgUpdateIsAuthorized(b *gotgbot.Bot, c *ext.Context) bool {
func TgRegisterBotCommands(b *gotgbot.Bot, commands ...gotgbot.BotCommand) error {

# Reply & Errors
func SendMessageConfirmation(
func TgSendTextById(b *gotgbot.Bot, chatId int64, threadId int64, text string) error {
func TgSendErrorById(b *gotgbot.Bot, chatId, threadId int64, eMessage string, e error) error {
func TgReplyTextByContext(b *gotgbot.Bot, c *ext.Context, text string, buttons *gotgbot.InlineKeyboardMarkup, silent bool) (*gotgbot.Message, error) {
func TgReplyWithErrorByContext(b *gotgbot.Bot, c *ext.Context, eMessage string, e error) error {

# Callback
func TgBuildUrlButton(text, url string) gotgbot.InlineKeyboardMarkup {
func TgMakeRevokeKeyboard(msgId, chatId string, confirm bool) *gotgbot.InlineKeyboardMarkup {
```
>>>>>>> Stashed changes
