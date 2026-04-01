FROM python:3.12-slim AS base

# Inject corporate proxy CA so Python's ssl module can verify external TLS (Discord, etc.).
# corp-ca.pem is machine-specific and gitignored. An empty placeholder is committed so this
# COPY always succeeds. The [ -s ] check skips installation when the file is empty (no proxy).
COPY corp-ca.pem /tmp/corp-ca.pem
RUN apt-get update \
  && apt-get install -y --no-install-recommends ca-certificates \
  && if [ -s /tmp/corp-ca.pem ]; then \
       cp /tmp/corp-ca.pem /usr/local/share/ca-certificates/corp-ca.crt \
       && update-ca-certificates; \
     fi \
  && rm -rf /var/lib/apt/lists/* /tmp/corp-ca.pem

WORKDIR /app

# Install dependencies first (layer cache).
# --trusted-host flags are harmless on machines without a TLS-intercepting proxy.
COPY requirements.txt .
RUN pip install --no-cache-dir \
    --trusted-host pypi.org \
    --trusted-host files.pythonhosted.org \
    --trusted-host pypi.python.org \
    -r requirements.txt

# Copy source
COPY . .

ENV PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1

CMD ["python", "main.py"]
