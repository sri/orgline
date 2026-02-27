type HelloResponse = {
  message: string;
};

const messageEl = document.getElementById("server-message");

async function loadServerMessage(): Promise<void> {
  if (!messageEl) {
    return;
  }

  try {
    const response = await fetch("/api/hello");
    if (!response.ok) {
      throw new Error(`request failed with status ${response.status}`);
    }

    const data = (await response.json()) as HelloResponse;
    messageEl.textContent = `Server says: ${data.message}`;
  } catch (error) {
    messageEl.textContent = `Unable to load server message: ${String(error)}`;
  }
}

void loadServerMessage();
