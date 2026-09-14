# Work on Yogilib from your phone and computers

For the latest reported progress and the next unfinished step, read [the project handoff](08-project-handoff.md). The steps below are setup instructions, not proof that all devices are connected.

Use this laptop as Yogilib's main working computer first. Connect your phone and other computer through Remote, then open the same Yogilib task to continue. This keeps the project, uploaded files, and existing setup together on the laptop while you give instructions from another device.

## Connect your phone

1. On this laptop, open the desktop app's **Settings → Connections → Control this Mac or PC → Set up** (or **Add**).
2. Approve the connection and scan the displayed QR code with your phone. Sign in with the same ChatGPT account and workspace and complete any verification.
3. In the ChatGPT mobile app, open **Remote**, choose this laptop, and open the Yogilib task. You can request changes, send follow-ups, and review the results there.

Keep the laptop plugged in, awake, online, and the desktop app running. On a Mac laptop, leave the lid open unless an external display is connected. Remote availability depends on app rollout and workspace settings; update both apps if the controls are missing.

## Continue on your other computer

On a supported Mac or Windows computer, sign in to the same account and workspace in the desktop app. Use **Settings → Connections → Control other devices** to connect this laptop, then open its existing Yogilib task. The work still runs on this laptop.

To check the setup, send this from your phone: “For Yogilib, tell me the current branch and latest commit without changing files.” Open that same task on the other computer and send: “Continue this task and summarize the project status.” Both messages should appear in the same conversation. Device setup is complete only after this works.

## If you want the other computer to run the work

This is a separate setup step. Save a copy of the same GitHub repository as a project on the destination computer, install Go 1.25 or newer, and configure its development database privately. Uploaded files under `static/docs` also need a separate transfer or shared storage. In the desktop app, use the task's run-location control to hand it to the connected host with the matching project. Verify the destination can run it before relying on this route.

For ordinary Git sync, ask the agent to check for changes from both computers, save work on a named branch, push it to GitHub, and report the branch and commit. On the next computer, ask it to retrieve that branch while preserving local changes. GitHub does not receive unsaved or unpushed changes automatically.

## What is ready, and what still needs checking

Checked September 14, 2026:

- The laptop project is `/Users/kafle/Desktop/yogilib`, connected to `git@github.com:helonmelon/yogilib.git`.
- Before this guide was added, the laptop was clean at `2dd8a99`, one commit ahead of the verified GitHub main branch at `af6f572`. That update was not on GitHub. This guide does not itself push changes.
- The project uses Neon Postgres. Uploaded files remain local under `static/docs` and are ignored by Git. Using the same connected laptop retains access to those files; cloning the repository alone does not.
- The user subsequently reported completing phone pairing and opening the setup task through Remote. Phone execution and access from the other computer still need verification; see the project handoff.
- Fly hosting files exist, but still reference the old SQLite setup. They do not establish that a working site is deployed. No deployment was performed.

## Reference

Official OpenAI documentation: [Remote setup](https://learn.chatgpt.com/docs/remote) and [Remote connections, supported devices, and handoff](https://learn.chatgpt.com/docs/remote-connections). These instructions describe the currently documented ChatGPT desktop/mobile labels; labels and availability in the installed app can differ.
