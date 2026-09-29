-- Create deployments table for infraX control plane
CREATE TABLE IF NOT EXISTS deployments (
  id SERIAL PRIMARY KEY,
  tenant TEXT NOT NULL,
  image TEXT NOT NULL,
  created_at TIMESTAMPTZ DEFAULT now()
);
