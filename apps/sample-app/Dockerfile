FROM node:18-alpine
WORKDIR /app

# Install production dependencies if any
COPY package.json package-lock.json* ./
RUN npm install --production || true

# Copy app sources
COPY . .

ENV PORT=8080
EXPOSE 8080
CMD ["node", "index.js"]
FROM node:18-alpine
WORKDIR /app
COPY package.json .
RUN npm ci --only=production || true
COPY index.js .
ENV PORT=8080
EXPOSE 8080
CMD ["node","index.js"]
