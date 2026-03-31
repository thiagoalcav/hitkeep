"use strict";

const { createBuilder } = require("@angular-devkit/architect");
const { spawn } = require("node:child_process");
const { resolve } = require("node:path");

async function runCommand(options, context) {
    const cwd = resolve(context.workspaceRoot, options.cwd ?? ".");

    context.logger.info(`Running e2e command: ${options.command}`);

    return new Promise((resolveResult) => {
        const child = spawn(options.command, {
            cwd,
            env: process.env,
            shell: true,
            stdio: "inherit"
        });

        child.once("error", (error) => {
            context.logger.error(error.message);
            resolveResult({
                error: error.message,
                success: false
            });
        });

        child.once("close", (code, signal) => {
            if (signal) {
                context.logger.error(`E2E command terminated with signal ${signal}.`);
                resolveResult({ success: false });
                return;
            }

            resolveResult({ success: code === 0 });
        });
    });
}

module.exports = createBuilder(runCommand);
