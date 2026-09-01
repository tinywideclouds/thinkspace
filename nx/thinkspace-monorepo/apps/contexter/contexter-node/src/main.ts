import express from 'express';
import cors from 'cors';
import { apiRouter } from './routes/api.routes';

const app = express();
app.use(cors());

// Increase the payload limit for large context bundles
app.use(express.json({ limit: '50mb' }));
app.use(express.urlencoded({ limit: '50mb', extended: true }));

// Mount the router
app.use('/api', apiRouter);

const port = process.env['PORT'] || 3333;
const server = app.listen(port, () => {
  console.log(`Listening at http://localhost:${port}/api`);
});
server.on('error', console.error);