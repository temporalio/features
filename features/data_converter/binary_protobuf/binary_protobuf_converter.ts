import * as proto from '@temporalio/proto';
import {
  DefaultPayloadConverterWithProtobufsOptions,
  ProtobufBinaryPayloadConverter,
} from '@temporalio/common/lib/protobufs';

import {
  UndefinedPayloadConverter,
  CompositePayloadConverter,
  BinaryPayloadConverter,
} from '@temporalio/common';

class payloadConverterWithProtobufs extends CompositePayloadConverter {
  constructor({ protobufRoot }: DefaultPayloadConverterWithProtobufsOptions) {
    super(
      new UndefinedPayloadConverter(),
      new BinaryPayloadConverter(),
      new ProtobufBinaryPayloadConverter(protobufRoot),
    );
  }
}

export const payloadConverter =  new payloadConverterWithProtobufs({
  protobufRoot: proto,
})
