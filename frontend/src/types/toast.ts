export type Toast = {
  id?: string;
  title?: string;
  type: 'success' | 'error' | 'warning' | 'info' | 'neutral',
  message: string;
};

export type Popup = {
  id?: string;
  title?: string;
}