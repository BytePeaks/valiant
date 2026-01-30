FROM node:20-alpine

WORKDIR /app

# Copy package.json and install dependencies
COPY package.json ./
# In a real scenario we'd copy package-lock.json too
RUN npm install

# Copy the rest of the application code
COPY . .

# Expose port 3000
EXPOSE 3000

# Start the application in dev mode for local development
CMD ["npm", "run", "dev"]
