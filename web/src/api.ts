let csrfToken = '';

export function setCSRFToken(token: string) {
  csrfToken = token;
}

export function formatUSD(microUSD?: number): string {
  if (microUSD === undefined || microUSD === null) return 'Unknown';
  const isNegative = microUSD < 0;
  const absVal = Math.abs(microUSD);
  const dollars = Math.floor(absVal / 1000000);
  const micros = absVal % 1000000;
  
  let formatted = '';
  if (micros === 0) {
    formatted = `$${dollars}.00`;
  } else if (micros % 10000 === 0) {
    formatted = `$${dollars}.${String(Math.floor(micros / 10000)).padStart(2, '0')}`;
  } else {
    let str = String(micros).padStart(6, '0').replace(/0+$/, '');
    if (str.length < 2) str = str.padEnd(2, '0');
    formatted = `$${dollars}.${str}`;
  }

  return isNegative ? `-${formatted}` : formatted;
}

export async function apiRequest<T>(path: string, options: RequestInit = {}): Promise<T> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(options.headers as Record<string, string> || {}),
  };

  if (csrfToken && (options.method === 'POST' || options.method === 'PUT' || options.method === 'DELETE')) {
    headers['X-CSRF-Token'] = csrfToken;
  }

  const res = await fetch(path, {
    ...options,
    headers,
    credentials: 'same-origin',
  });

  if (!res.ok) {
    let errMsg = `Request failed (${res.status})`;
    try {
      const errJson = await res.json();
      if (errJson.error) errMsg = errJson.error;
    } catch {}
    throw new Error(errMsg);
  }

  return res.json();
}
