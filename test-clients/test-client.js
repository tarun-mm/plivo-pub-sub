#!/usr/bin/env node

/**
 * WebSocket Test Client for PubSub System
 *
 * Usage: node test-client.js [server-url]
 * Example: node test-client.js ws://localhost:8080
 *
 * Commands:
 *   sub <topic> [last_n]  - Subscribe to topic
 *   unsub <topic>         - Unsubscribe from topic
 *   pub <topic> <data>    - Publish message to topic
 *   ping                  - Send ping to server
 *   quit                  - Exit client
 */

const WebSocket = require('ws');
const readline = require('readline');
const { v4: uuidv4 } = require('uuid');

// Parse command line arguments
const serverUrl = process.argv[2] || 'ws://localhost:8080';
const clientId = `test-client-${Math.random().toString(36).substr(2, 9)}`;

// Colors for terminal output
const colors = {
  reset: '\x1b[0m',
  bright: '\x1b[1m',
  dim: '\x1b[2m',
  red: '\x1b[31m',
  green: '\x1b[32m',
  yellow: '\x1b[33m',
  blue: '\x1b[34m',
  magenta: '\x1b[35m',
  cyan: '\x1b[36m',
};

// Create WebSocket connection
const wsUrl = `${serverUrl}/ws?client_id=${clientId}`;
console.log(`${colors.cyan}Connecting to: ${wsUrl}${colors.reset}`);

const ws = new WebSocket(wsUrl);

// Create readline interface
const rl = readline.createInterface({
  input: process.stdin,
  output: process.stdout,
  prompt: `${colors.green}>${colors.reset} `
});

// WebSocket event handlers
ws.on('open', () => {
  console.log(`${colors.green}✓ Connected to server${colors.reset}`);
  console.log(`${colors.cyan}Client ID: ${clientId}${colors.reset}\n`);
  printHelp();
  rl.prompt();
});

ws.on('message', (data) => {
  try {
    const msg = JSON.parse(data.toString());
    printServerMessage(msg);
  } catch (error) {
    console.error(`${colors.red}Failed to parse message:${colors.reset}`, error);
  }
  rl.prompt();
});

ws.on('close', (code, reason) => {
  console.log(`\n${colors.yellow}Disconnected from server${colors.reset}`);
  console.log(`Code: ${code}, Reason: ${reason || 'N/A'}`);
  process.exit(0);
});

ws.on('error', (error) => {
  console.error(`${colors.red}WebSocket error:${colors.reset}`, error.message);
  process.exit(1);
});

// Handle user input
rl.on('line', (input) => {
  const trimmed = input.trim();
  if (trimmed) {
    handleCommand(trimmed);
  } else {
    rl.prompt();
  }
});

rl.on('close', () => {
  console.log(`\n${colors.yellow}Exiting...${colors.reset}`);
  ws.close();
  process.exit(0);
});

// Command handler
function handleCommand(input) {
  const parts = input.split(/\s+/);
  const cmd = parts[0].toLowerCase();

  try {
    let message;

    switch (cmd) {
      case 'sub':
      case 'subscribe':
        if (!parts[1]) {
          console.log(`${colors.red}Usage: sub <topic> [last_n]${colors.reset}`);
          break;
        }
        message = {
          type: 'subscribe',
          topic: parts[1],
          client_id: clientId,
          last_n: parseInt(parts[2]) || 0,
          request_id: uuidv4()
        };
        sendMessage(message);
        break;

      case 'unsub':
      case 'unsubscribe':
        if (!parts[1]) {
          console.log(`${colors.red}Usage: unsub <topic>${colors.reset}`);
          break;
        }
        message = {
          type: 'unsubscribe',
          topic: parts[1],
          client_id: clientId,
          request_id: uuidv4()
        };
        sendMessage(message);
        break;

      case 'pub':
      case 'publish':
        if (!parts[1] || !parts[2]) {
          console.log(`${colors.red}Usage: pub <topic> <data>${colors.reset}`);
          break;
        }
        const payload = parts.slice(2).join(' ');
        let parsedPayload;
        try {
          parsedPayload = JSON.parse(payload);
        } catch {
          parsedPayload = payload;
        }
        message = {
          type: 'publish',
          topic: parts[1],
          message: {
            id: uuidv4(),
            payload: parsedPayload
          },
          request_id: uuidv4()
        };
        sendMessage(message);
        break;

      case 'ping':
        message = {
          type: 'ping',
          request_id: uuidv4()
        };
        sendMessage(message);
        break;

      case 'help':
      case '?':
        printHelp();
        break;

      case 'quit':
      case 'exit':
        ws.close();
        break;

      default:
        console.log(`${colors.red}Unknown command: ${cmd}${colors.reset}`);
        console.log(`${colors.dim}Type 'help' for available commands${colors.reset}`);
    }
  } catch (error) {
    console.error(`${colors.red}Error handling command:${colors.reset}`, error.message);
  }

  rl.prompt();
}

// Send message to server
function sendMessage(message) {
  if (ws.readyState === WebSocket.OPEN) {
    console.log(`${colors.blue}>> Sending:${colors.reset}`, JSON.stringify(message, null, 2));
    ws.send(JSON.stringify(message));
  } else {
    console.log(`${colors.red}WebSocket is not open. Current state: ${ws.readyState}${colors.reset}`);
  }
}

// Print server message
function printServerMessage(msg) {
  const timestamp = new Date(msg.ts).toLocaleTimeString();

  switch (msg.type) {
    case 'ack':
      console.log(`${colors.green}<< [${timestamp}] ACK:${colors.reset} ${msg.topic || 'N/A'} - ${msg.status}`);
      break;

    case 'event':
      console.log(`${colors.cyan}<< [${timestamp}] EVENT:${colors.reset} topic=${msg.topic}`);
      console.log(`   Message ID: ${msg.message.id}`);
      console.log(`   Payload:`, JSON.stringify(msg.message.payload, null, 2));
      break;

    case 'error':
      console.log(`${colors.red}<< [${timestamp}] ERROR:${colors.reset} ${msg.error.code}`);
      console.log(`   ${msg.error.message}`);
      break;

    case 'pong':
      console.log(`${colors.magenta}<< [${timestamp}] PONG${colors.reset}`);
      break;

    case 'info':
      console.log(`${colors.yellow}<< [${timestamp}] INFO:${colors.reset} ${msg.msg} ${msg.topic ? `(topic: ${msg.topic})` : ''}`);
      break;

    default:
      console.log(`${colors.dim}<< [${timestamp}]${colors.reset}`, JSON.stringify(msg, null, 2));
  }
}

// Print help
function printHelp() {
  console.log(`${colors.bright}Available Commands:${colors.reset}`);
  console.log(`  ${colors.cyan}sub <topic> [last_n]${colors.reset}  - Subscribe to topic (optional: get last N messages)`);
  console.log(`  ${colors.cyan}unsub <topic>${colors.reset}         - Unsubscribe from topic`);
  console.log(`  ${colors.cyan}pub <topic> <data>${colors.reset}    - Publish message to topic`);
  console.log(`  ${colors.cyan}ping${colors.reset}                  - Send ping to server`);
  console.log(`  ${colors.cyan}help${colors.reset}                  - Show this help`);
  console.log(`  ${colors.cyan}quit${colors.reset}                  - Exit client\n`);
  console.log(`${colors.dim}Example:${colors.reset}`);
  console.log(`  sub orders 5`);
  console.log(`  pub orders {"order_id":"123","amount":99.5}`);
  console.log(`  pub orders hello-world`);
  console.log(``);
}

// Handle process signals
process.on('SIGINT', () => {
  console.log(`\n${colors.yellow}Received SIGINT, closing connection...${colors.reset}`);
  ws.close();
  process.exit(0);
});
