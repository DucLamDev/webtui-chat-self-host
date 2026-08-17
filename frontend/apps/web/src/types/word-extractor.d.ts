declare module "word-extractor/lib/word-ole-extractor" {
  import type { Buffer } from "buffer";

  interface WordExtractorDocument {
    getBody(): string;
  }

  interface WordExtractorReader {
    open(): Promise<void>;
    close(): Promise<void>;
    read(buffer: Buffer, offset: number, length: number, position: number): Promise<Buffer>;
    buffer(): Buffer;
  }

  class WordOleExtractor {
    extract(reader: WordExtractorReader): Promise<WordExtractorDocument>;
  }

  export = WordOleExtractor;
}

declare module "word-extractor/lib/buffer-reader" {
  import type { Buffer } from "buffer";

  class BufferReader {
    constructor(buffer: Buffer);
    open(): Promise<void>;
    close(): Promise<void>;
    read(buffer: Buffer, offset: number, length: number, position: number): Promise<Buffer>;
    buffer(): Buffer;
    static isBufferReader(instance: unknown): instance is BufferReader;
  }

  export = BufferReader;
}
