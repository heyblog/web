import QRCode from 'qrcode';

export function renderSiteQr(url: string): string {
  const { modules } = QRCode.create(url, { errorCorrectionLevel: 'M' });
  const quiet = 4;
  const cell = Math.floor(200 / (modules.size + quiet * 2));
  const size = (modules.size + quiet * 2) * cell;
  const left = Math.round(1020 - size / 2);
  const top = Math.round(330 - size / 2);
  const paths: string[] = [];
  for (let row = 0; row < modules.size; row += 1) {
    for (let column = 0; column < modules.size; column += 1) {
      if (modules.get(row, column)) {
        const x = left + (column + quiet) * cell;
        const y = top + (row + quiet) * cell;
        paths.push(`M${x} ${y}h${cell}v${cell}h-${cell}z`);
      }
    }
  }
  return `<rect x="${left}" y="${top}" width="${size}" height="${size}" fill="#FFFFFF"/>
    <path d="${paths.join('')}" fill="#241E18" shape-rendering="crispEdges"/>`;
}
