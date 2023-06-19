import * as proto from '@temporalio/proto';
import {
  DefaultPayloadConverterWithProtobufsOptions,
  ProtobufJsonPayloadConverter,
} from '@temporalio/common/lib/protobufs';

import { CompositePayloadConverter } from '@temporalio/common';

class PayloadConverterWithProtobufs extends CompositePayloadConverter {
  constructor({ protobufRoot }: DefaultPayloadConverterWithProtobufsOptions) {
    super(new ProtobufJsonPayloadConverter(protobufRoot));
  }
}

export const payloadConverter = new PayloadConverterWithProtobufs({
  protobufRoot: proto,
});
