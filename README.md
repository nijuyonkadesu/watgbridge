# WhatsApp-Telegram-Bridge

Despite the name, its not exactly a "bridge". It forwards messages from WhatsApp to Telegram and you can reply to them
from Telegram.

<a href="https://t.me/PropheCProjects">
  <img src="https://img.shields.io/badge/Updates_Channel-2CA5E0?style=for-the-badge&logo=telegram&logoColor=white"></img>
</a>&nbsp; &nbsp;
<a href="https://t.me/WaTgBridge">
  <img src="https://img.shields.io/badge/Discussion_Group-2CA5E0?style=for-the-badge&logo=telegram&logoColor=white"></img>
</a>&nbsp; &nbsp;
<a href="https://youtu.be/xc75XLoTmA4">
  <img src="https://img.shields.io/badge/YouTube-FF0000?style=for-the-badge&logo=youtube&logoColor=white"</img>
</a>

# DISCLAIMER !!!

This project is in no way affiliated with WhatsApp or Telegram. Using this can also lead to your account getting banned by WhatsApp so use at your own risk.

## Sample Screenshots

<p align="center">
  <img src="./assets/telegram_side_sample.png" width="350" alt="Telegram Side">
  <img src="./assets/whatsapp_side_sample.jpg" width="350" alt="WhatsApp Side">
</p>

## Features and Design Choices

- All messages from various chats (on WhatsApp) are sent to different topics/threads within the same target group (on Telegram)
- Configuration options available to disable different types of updates from WhatsApp
- Can reply and send new messages from Telegram
- Can tag all people using @all or @everyone. Others can also use this in group chats which you specify in configuration file
- Can react to messages by replying with single instance of the desired emoji
- Supports static stickers from both ends
- Can send Animated (TGS) stickers from Telegram
- Video stickers from Telegram side are supported
- Video stickers from WhatsApp side are currently forwarded as GIFs to Telegram

## Bugs and TODO

- Document naming is messed up and not consistent on Telegram, have to find a way to always send same names

PRs are welcome :)

## Installation

- Make a supergroup (enable message history for new members) with topics enabled
- Add your bot in the group, make it an admin with permissions to `Manage topics`
- Install `git`, `gcc` and `golang`, `ffmpeg` , `imagemagick` (optional), on your system
- Clone this repository anywhere and navigate to the cloned directory
- Run `go build`
- Copy `sample_config.yaml` to `config.yaml` and fill the values, there are comments to help you.
- Execute the binary by running `./watgbridge`
- On first run, it will show QR code for logging into WhatsApp that can by scanned by the WhatsApp app in `Linked devices`
- It is recommended to restart the bot after every few hours becuase WhatsApp likes to disconnect a lot. So a sample Systemd service file has been provided (`watgbridge.service.sample`). Edit the `User` and `ExecStart` according to your setup:
  - If you do not have local bot API server, remove `tgbotapi.service` from the `After` key in `Unit` section.
  - This service file will restart the bot every 24 hours

# Systemd Service

- place the file in `etc/systemd/system/` with your user scope
- have to check if telegram-bot-api.service's /data & /temp 770 permission
- start your database

- TODO: watgbride dockerfile, FROM scratch, copy the binary, mount tg volumes
- so now it's isolated and doesn't matter if runs in root

# Playground

```sh
sudo nvim /etc/containers/homelab/portable-postgres.env
sudo chmod 770 /etc/containers/homelab/portable-postgres.env
sudo cp portable-postgres.container /etc/containers/systemd/
/usr/lib/systemd/system-generators/podman-system-generator --dryrun

sudo podman ps

# cloning postgres
pg_dump -U guts -h localhost -p 5432 watgbridge | psql -U guts -h ustable -d watgbridge
scp -P 8022 u0_a342@watgtunnel:~/watgbridge/wawebstore.db ~/redacted/watgbridge
scp -P 8022 u0_a342@watgtunnel:~/watgbridge/config.yaml ~/redacted/watgbridge

# copy watgbridge.service
sudo cp watgbridge.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable watgbridge.service
sudo systemctl start watgbridge.service

journalctl -u watgbridge.service -f
```

### portable-postgres.env

```
POSTGRES_USER=uname
POSTGRES_PASSWORD=pass
POSTGRES_DB=watgbridge
```

### Dependencies

1. imagemagick
2. webp (webpmux)
3. git
4. ffmpeg

### Before May End


[TODO] understand database, gorm's features, and migration
[TODO] Fix live loc (maybe use with config debug mode?)
[TODO] Create a PR

[TODO] understand modules
[TODO] TgSendToWhatsApp - separate all media handlers into a separate individual units (not more than 300 lines in each file)
[TODO] how did memory doubled? - https://t.me/WaTgBridge/11036

#### Suspected bugs

[TODO] Fix audio issue
[TODO] unlinkthread - bug?
[TODO] 2x memory consumption for big media files

# Improvements:

what's a stanza ID? maybe, try to figure out data model tomorrow?
[TODO] what's a stanza ID? maybe, try to figure out data model tomorrow?
[TODO] what's a participantID?
[TODO] dam... `ClearMessageIdPairsHistoryHandler` really deletes everything...
[TODO] check does bot receives all updates from whatsapp or not - find it's impl
[TODO] better logs, multiline tracebacks (do not mix with logs?)
[TODO] cache for recent stickers?
[TODO] route bot messages
[TODO] move sqlite to postgres
[TODO] backup sql
[TODO] 300kb (time) sticker got blew up to 3.4MB, and literally minutes of processing. understand what's happening and optimize them
[TODO] propagate quotes

2025-06-21

1. dispatcher.AddGroupHandlers

   - (-1) for middleware, is allowed first of all
   - (0) once allowed - then check for admin? or check for command? - I think it's not configured in the best way in watgbride??
   - (10+) for application logic??

2. BridgeTelegramToWhatsAppHandler.TgUpdateIsAuthorized - move this as a middleware instead of checking in telegram utils - dayum, 19 references.

## NewModules

1. demotivator module [Frame + centralized caption]
   - add russian translation for authentic flavour
2. media download module - check that go mod
3. regex substitute module
4. LLM Interaction Module (with #llm flag or smth) - save llms and choose based on callback or smth
