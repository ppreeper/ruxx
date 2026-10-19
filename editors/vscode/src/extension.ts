import * as fs from 'fs';
import * as path from 'path';
import * as vscode from 'vscode';
import {
	Executable,
	LanguageClient,
	LanguageClientOptions,
	ServerOptions,
	TransportKind,
} from 'vscode-languageclient/node';

let client: LanguageClient | undefined;

/**
 * Locate the ruxx binary: an explicit `ruxx.binaryPath` wins, then a binary
 * shipped next to the extension, then whatever `ruxx` resolves to on PATH.
 */
function resolveBinary(): string {
	const configured = vscode.workspace.getConfiguration('ruxx').get<string>('binaryPath', '').trim();
	if (configured.length > 0) {
		return configured;
	}
	// A platform-specific package ships the matching binary at the extension
	// root (ruxx.exe on Windows).
	const names = process.platform === 'win32' ? ['ruxx.exe', 'ruxx'] : ['ruxx'];
	for (const name of names) {
		const bundled = path.join(__dirname, '..', name);
		if (fs.existsSync(bundled)) {
			return bundled;
		}
	}
	return 'ruxx';
}

function serverOptions(): ServerOptions {
	const args = ['server'];
	const configPath = vscode.workspace.getConfiguration('ruxx').get<string>('configPath', '').trim();
	if (configPath.length > 0) {
		args.push('--config', configPath);
	}
	const executable: Executable = {
		command: resolveBinary(),
		args,
		transport: TransportKind.stdio,
	};
	return { run: executable, debug: executable };
}

function clientOptions(): LanguageClientOptions {
	return {
		// Same selector the CLI covers: every XML file, Odoo-specific rules are
		// gated by path inside the server.
		documentSelector: [{ scheme: 'file', language: 'xml' }],
		outputChannelName: 'ruxx',
	};
}

export async function activate(context: vscode.ExtensionContext): Promise<void> {
	client = new LanguageClient('ruxx', 'ruxx', serverOptions(), clientOptions());

	context.subscriptions.push(
		vscode.commands.registerCommand('ruxx.restartServer', async () => {
			await restart();
		}),
		vscode.commands.registerCommand('ruxx.showOutput', () => {
			client?.outputChannel.show(true);
		}),
		// A settings change (binary path, --config) needs a fresh server.
		vscode.workspace.onDidChangeConfiguration(async (event) => {
			if (event.affectsConfiguration('ruxx.binaryPath') || event.affectsConfiguration('ruxx.configPath')) {
				await restart();
			}
		}),
	);

	try {
		await client.start();
	} catch (err) {
		const message = err instanceof Error ? err.message : String(err);
		const choice = await vscode.window.showErrorMessage(
			`ruxx: the language server failed to start (${message}). Is the ruxx binary installed?`,
			'Show output',
			'Open settings',
		);
		if (choice === 'Show output') {
			client.outputChannel.show(true);
		} else if (choice === 'Open settings') {
			await vscode.commands.executeCommand('workbench.action.openSettings', 'ruxx.binaryPath');
		}
	}
}

async function restart(): Promise<void> {
	if (!client) {
		return;
	}
	await client.stop();
	client = new LanguageClient('ruxx', 'ruxx', serverOptions(), clientOptions());
	await client.start();
}

export async function deactivate(): Promise<void> {
	if (client) {
		await client.stop();
		client = undefined;
	}
}
