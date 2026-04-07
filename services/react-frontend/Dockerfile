FROM node:22-alpine

WORKDIR /app
COPY services/react-frontend/package.json /app/package.json
COPY services/react-frontend/tsconfig.json /app/tsconfig.json
COPY services/react-frontend/tsconfig.app.json /app/tsconfig.app.json
COPY services/react-frontend/tsconfig.node.json /app/tsconfig.node.json
COPY services/react-frontend/vite.config.ts /app/vite.config.ts
COPY services/react-frontend/index.html /app/index.html
COPY services/react-frontend/src /app/src
RUN npm install

EXPOSE 5173
CMD ["npm", "run", "dev"]
