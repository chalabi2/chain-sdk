// @generated
impl serde::Serialize for ExportChunk {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.data.is_empty() {
            len += 1;
        }
        if !self.sha256.is_empty() {
            len += 1;
        }
        if self.offset != 0 {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("akash.volume.v1.ExportChunk", len)?;
        if !self.data.is_empty() {
            #[allow(clippy::needless_borrow)]
            #[allow(clippy::needless_borrows_for_generic_args)]
            struct_ser.serialize_field("data", pbjson::private::base64::encode(&self.data).as_str())?;
        }
        if !self.sha256.is_empty() {
            #[allow(clippy::needless_borrow)]
            #[allow(clippy::needless_borrows_for_generic_args)]
            struct_ser.serialize_field("sha256", pbjson::private::base64::encode(&self.sha256).as_str())?;
        }
        if self.offset != 0 {
            #[allow(clippy::needless_borrow)]
            #[allow(clippy::needless_borrows_for_generic_args)]
            struct_ser.serialize_field("offset", ToString::to_string(&self.offset).as_str())?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for ExportChunk {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "data",
            "sha256",
            "offset",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Data,
            Sha256,
            Offset,
        }
        impl<'de> serde::Deserialize<'de> for GeneratedField {
            fn deserialize<D>(deserializer: D) -> std::result::Result<GeneratedField, D::Error>
            where
                D: serde::Deserializer<'de>,
            {
                struct GeneratedVisitor;

                impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
                    type Value = GeneratedField;

                    fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                        write!(formatter, "expected one of: {:?}", &FIELDS)
                    }

                    #[allow(unused_variables)]
                    fn visit_str<E>(self, value: &str) -> std::result::Result<GeneratedField, E>
                    where
                        E: serde::de::Error,
                    {
                        match value {
                            "data" => Ok(GeneratedField::Data),
                            "sha256" => Ok(GeneratedField::Sha256),
                            "offset" => Ok(GeneratedField::Offset),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = ExportChunk;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct akash.volume.v1.ExportChunk")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<ExportChunk, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut data__ = None;
                let mut sha256__ = None;
                let mut offset__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Data => {
                            if data__.is_some() {
                                return Err(serde::de::Error::duplicate_field("data"));
                            }
                            data__ = 
                                Some(map_.next_value::<::pbjson::private::BytesDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::Sha256 => {
                            if sha256__.is_some() {
                                return Err(serde::de::Error::duplicate_field("sha256"));
                            }
                            sha256__ = 
                                Some(map_.next_value::<::pbjson::private::BytesDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::Offset => {
                            if offset__.is_some() {
                                return Err(serde::de::Error::duplicate_field("offset"));
                            }
                            offset__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                    }
                }
                Ok(ExportChunk {
                    data: data__.unwrap_or_default(),
                    sha256: sha256__.unwrap_or_default(),
                    offset: offset__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("akash.volume.v1.ExportChunk", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for ExportObjectsRequest {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.owner.is_empty() {
            len += 1;
        }
        if !self.vid.is_empty() {
            len += 1;
        }
        if self.dseq != 0 {
            len += 1;
        }
        if !self.requester.is_empty() {
            len += 1;
        }
        if !self.start_after.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("akash.volume.v1.ExportObjectsRequest", len)?;
        if !self.owner.is_empty() {
            struct_ser.serialize_field("owner", &self.owner)?;
        }
        if !self.vid.is_empty() {
            struct_ser.serialize_field("vid", &self.vid)?;
        }
        if self.dseq != 0 {
            #[allow(clippy::needless_borrow)]
            #[allow(clippy::needless_borrows_for_generic_args)]
            struct_ser.serialize_field("dseq", ToString::to_string(&self.dseq).as_str())?;
        }
        if !self.requester.is_empty() {
            struct_ser.serialize_field("requester", &self.requester)?;
        }
        if !self.start_after.is_empty() {
            struct_ser.serialize_field("startAfter", &self.start_after)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for ExportObjectsRequest {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "owner",
            "vid",
            "dseq",
            "requester",
            "start_after",
            "startAfter",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Owner,
            Vid,
            Dseq,
            Requester,
            StartAfter,
        }
        impl<'de> serde::Deserialize<'de> for GeneratedField {
            fn deserialize<D>(deserializer: D) -> std::result::Result<GeneratedField, D::Error>
            where
                D: serde::Deserializer<'de>,
            {
                struct GeneratedVisitor;

                impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
                    type Value = GeneratedField;

                    fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                        write!(formatter, "expected one of: {:?}", &FIELDS)
                    }

                    #[allow(unused_variables)]
                    fn visit_str<E>(self, value: &str) -> std::result::Result<GeneratedField, E>
                    where
                        E: serde::de::Error,
                    {
                        match value {
                            "owner" => Ok(GeneratedField::Owner),
                            "vid" => Ok(GeneratedField::Vid),
                            "dseq" => Ok(GeneratedField::Dseq),
                            "requester" => Ok(GeneratedField::Requester),
                            "startAfter" | "start_after" => Ok(GeneratedField::StartAfter),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = ExportObjectsRequest;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct akash.volume.v1.ExportObjectsRequest")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<ExportObjectsRequest, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut owner__ = None;
                let mut vid__ = None;
                let mut dseq__ = None;
                let mut requester__ = None;
                let mut start_after__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Owner => {
                            if owner__.is_some() {
                                return Err(serde::de::Error::duplicate_field("owner"));
                            }
                            owner__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Vid => {
                            if vid__.is_some() {
                                return Err(serde::de::Error::duplicate_field("vid"));
                            }
                            vid__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Dseq => {
                            if dseq__.is_some() {
                                return Err(serde::de::Error::duplicate_field("dseq"));
                            }
                            dseq__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::Requester => {
                            if requester__.is_some() {
                                return Err(serde::de::Error::duplicate_field("requester"));
                            }
                            requester__ = Some(map_.next_value()?);
                        }
                        GeneratedField::StartAfter => {
                            if start_after__.is_some() {
                                return Err(serde::de::Error::duplicate_field("startAfter"));
                            }
                            start_after__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(ExportObjectsRequest {
                    owner: owner__.unwrap_or_default(),
                    vid: vid__.unwrap_or_default(),
                    dseq: dseq__.unwrap_or_default(),
                    requester: requester__.unwrap_or_default(),
                    start_after: start_after__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("akash.volume.v1.ExportObjectsRequest", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for ExportRequest {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.owner.is_empty() {
            len += 1;
        }
        if !self.vid.is_empty() {
            len += 1;
        }
        if self.dseq != 0 {
            len += 1;
        }
        if !self.requester.is_empty() {
            len += 1;
        }
        if !self.from_snapshot.is_empty() {
            len += 1;
        }
        if self.resume_offset != 0 {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("akash.volume.v1.ExportRequest", len)?;
        if !self.owner.is_empty() {
            struct_ser.serialize_field("owner", &self.owner)?;
        }
        if !self.vid.is_empty() {
            struct_ser.serialize_field("vid", &self.vid)?;
        }
        if self.dseq != 0 {
            #[allow(clippy::needless_borrow)]
            #[allow(clippy::needless_borrows_for_generic_args)]
            struct_ser.serialize_field("dseq", ToString::to_string(&self.dseq).as_str())?;
        }
        if !self.requester.is_empty() {
            struct_ser.serialize_field("requester", &self.requester)?;
        }
        if !self.from_snapshot.is_empty() {
            struct_ser.serialize_field("fromSnapshot", &self.from_snapshot)?;
        }
        if self.resume_offset != 0 {
            #[allow(clippy::needless_borrow)]
            #[allow(clippy::needless_borrows_for_generic_args)]
            struct_ser.serialize_field("resumeOffset", ToString::to_string(&self.resume_offset).as_str())?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for ExportRequest {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "owner",
            "vid",
            "dseq",
            "requester",
            "from_snapshot",
            "fromSnapshot",
            "resume_offset",
            "resumeOffset",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Owner,
            Vid,
            Dseq,
            Requester,
            FromSnapshot,
            ResumeOffset,
        }
        impl<'de> serde::Deserialize<'de> for GeneratedField {
            fn deserialize<D>(deserializer: D) -> std::result::Result<GeneratedField, D::Error>
            where
                D: serde::Deserializer<'de>,
            {
                struct GeneratedVisitor;

                impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
                    type Value = GeneratedField;

                    fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                        write!(formatter, "expected one of: {:?}", &FIELDS)
                    }

                    #[allow(unused_variables)]
                    fn visit_str<E>(self, value: &str) -> std::result::Result<GeneratedField, E>
                    where
                        E: serde::de::Error,
                    {
                        match value {
                            "owner" => Ok(GeneratedField::Owner),
                            "vid" => Ok(GeneratedField::Vid),
                            "dseq" => Ok(GeneratedField::Dseq),
                            "requester" => Ok(GeneratedField::Requester),
                            "fromSnapshot" | "from_snapshot" => Ok(GeneratedField::FromSnapshot),
                            "resumeOffset" | "resume_offset" => Ok(GeneratedField::ResumeOffset),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = ExportRequest;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct akash.volume.v1.ExportRequest")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<ExportRequest, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut owner__ = None;
                let mut vid__ = None;
                let mut dseq__ = None;
                let mut requester__ = None;
                let mut from_snapshot__ = None;
                let mut resume_offset__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Owner => {
                            if owner__.is_some() {
                                return Err(serde::de::Error::duplicate_field("owner"));
                            }
                            owner__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Vid => {
                            if vid__.is_some() {
                                return Err(serde::de::Error::duplicate_field("vid"));
                            }
                            vid__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Dseq => {
                            if dseq__.is_some() {
                                return Err(serde::de::Error::duplicate_field("dseq"));
                            }
                            dseq__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::Requester => {
                            if requester__.is_some() {
                                return Err(serde::de::Error::duplicate_field("requester"));
                            }
                            requester__ = Some(map_.next_value()?);
                        }
                        GeneratedField::FromSnapshot => {
                            if from_snapshot__.is_some() {
                                return Err(serde::de::Error::duplicate_field("fromSnapshot"));
                            }
                            from_snapshot__ = Some(map_.next_value()?);
                        }
                        GeneratedField::ResumeOffset => {
                            if resume_offset__.is_some() {
                                return Err(serde::de::Error::duplicate_field("resumeOffset"));
                            }
                            resume_offset__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                    }
                }
                Ok(ExportRequest {
                    owner: owner__.unwrap_or_default(),
                    vid: vid__.unwrap_or_default(),
                    dseq: dseq__.unwrap_or_default(),
                    requester: requester__.unwrap_or_default(),
                    from_snapshot: from_snapshot__.unwrap_or_default(),
                    resume_offset: resume_offset__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("akash.volume.v1.ExportRequest", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for ObjectAttributes {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if self.size != 0 {
            len += 1;
        }
        if !self.content_type.is_empty() {
            len += 1;
        }
        if !self.content_encoding.is_empty() {
            len += 1;
        }
        if !self.content_disposition.is_empty() {
            len += 1;
        }
        if !self.content_language.is_empty() {
            len += 1;
        }
        if !self.cache_control.is_empty() {
            len += 1;
        }
        if !self.expires.is_empty() {
            len += 1;
        }
        if !self.metadata.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("akash.volume.v1.ObjectAttributes", len)?;
        if self.size != 0 {
            #[allow(clippy::needless_borrow)]
            #[allow(clippy::needless_borrows_for_generic_args)]
            struct_ser.serialize_field("size", ToString::to_string(&self.size).as_str())?;
        }
        if !self.content_type.is_empty() {
            struct_ser.serialize_field("contentType", &self.content_type)?;
        }
        if !self.content_encoding.is_empty() {
            struct_ser.serialize_field("contentEncoding", &self.content_encoding)?;
        }
        if !self.content_disposition.is_empty() {
            struct_ser.serialize_field("contentDisposition", &self.content_disposition)?;
        }
        if !self.content_language.is_empty() {
            struct_ser.serialize_field("contentLanguage", &self.content_language)?;
        }
        if !self.cache_control.is_empty() {
            struct_ser.serialize_field("cacheControl", &self.cache_control)?;
        }
        if !self.expires.is_empty() {
            struct_ser.serialize_field("expires", &self.expires)?;
        }
        if !self.metadata.is_empty() {
            struct_ser.serialize_field("metadata", &self.metadata)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for ObjectAttributes {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "size",
            "content_type",
            "contentType",
            "content_encoding",
            "contentEncoding",
            "content_disposition",
            "contentDisposition",
            "content_language",
            "contentLanguage",
            "cache_control",
            "cacheControl",
            "expires",
            "metadata",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Size,
            ContentType,
            ContentEncoding,
            ContentDisposition,
            ContentLanguage,
            CacheControl,
            Expires,
            Metadata,
        }
        impl<'de> serde::Deserialize<'de> for GeneratedField {
            fn deserialize<D>(deserializer: D) -> std::result::Result<GeneratedField, D::Error>
            where
                D: serde::Deserializer<'de>,
            {
                struct GeneratedVisitor;

                impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
                    type Value = GeneratedField;

                    fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                        write!(formatter, "expected one of: {:?}", &FIELDS)
                    }

                    #[allow(unused_variables)]
                    fn visit_str<E>(self, value: &str) -> std::result::Result<GeneratedField, E>
                    where
                        E: serde::de::Error,
                    {
                        match value {
                            "size" => Ok(GeneratedField::Size),
                            "contentType" | "content_type" => Ok(GeneratedField::ContentType),
                            "contentEncoding" | "content_encoding" => Ok(GeneratedField::ContentEncoding),
                            "contentDisposition" | "content_disposition" => Ok(GeneratedField::ContentDisposition),
                            "contentLanguage" | "content_language" => Ok(GeneratedField::ContentLanguage),
                            "cacheControl" | "cache_control" => Ok(GeneratedField::CacheControl),
                            "expires" => Ok(GeneratedField::Expires),
                            "metadata" => Ok(GeneratedField::Metadata),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = ObjectAttributes;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct akash.volume.v1.ObjectAttributes")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<ObjectAttributes, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut size__ = None;
                let mut content_type__ = None;
                let mut content_encoding__ = None;
                let mut content_disposition__ = None;
                let mut content_language__ = None;
                let mut cache_control__ = None;
                let mut expires__ = None;
                let mut metadata__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Size => {
                            if size__.is_some() {
                                return Err(serde::de::Error::duplicate_field("size"));
                            }
                            size__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::ContentType => {
                            if content_type__.is_some() {
                                return Err(serde::de::Error::duplicate_field("contentType"));
                            }
                            content_type__ = Some(map_.next_value()?);
                        }
                        GeneratedField::ContentEncoding => {
                            if content_encoding__.is_some() {
                                return Err(serde::de::Error::duplicate_field("contentEncoding"));
                            }
                            content_encoding__ = Some(map_.next_value()?);
                        }
                        GeneratedField::ContentDisposition => {
                            if content_disposition__.is_some() {
                                return Err(serde::de::Error::duplicate_field("contentDisposition"));
                            }
                            content_disposition__ = Some(map_.next_value()?);
                        }
                        GeneratedField::ContentLanguage => {
                            if content_language__.is_some() {
                                return Err(serde::de::Error::duplicate_field("contentLanguage"));
                            }
                            content_language__ = Some(map_.next_value()?);
                        }
                        GeneratedField::CacheControl => {
                            if cache_control__.is_some() {
                                return Err(serde::de::Error::duplicate_field("cacheControl"));
                            }
                            cache_control__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Expires => {
                            if expires__.is_some() {
                                return Err(serde::de::Error::duplicate_field("expires"));
                            }
                            expires__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Metadata => {
                            if metadata__.is_some() {
                                return Err(serde::de::Error::duplicate_field("metadata"));
                            }
                            metadata__ = Some(
                                map_.next_value::<std::collections::HashMap<_, _>>()?
                            );
                        }
                    }
                }
                Ok(ObjectAttributes {
                    size: size__.unwrap_or_default(),
                    content_type: content_type__.unwrap_or_default(),
                    content_encoding: content_encoding__.unwrap_or_default(),
                    content_disposition: content_disposition__.unwrap_or_default(),
                    content_language: content_language__.unwrap_or_default(),
                    cache_control: cache_control__.unwrap_or_default(),
                    expires: expires__.unwrap_or_default(),
                    metadata: metadata__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("akash.volume.v1.ObjectAttributes", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for ObjectChunk {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.key.is_empty() {
            len += 1;
        }
        if self.attributes.is_some() {
            len += 1;
        }
        if self.offset != 0 {
            len += 1;
        }
        if !self.data.is_empty() {
            len += 1;
        }
        if !self.sha256.is_empty() {
            len += 1;
        }
        if !self.object_sha256.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("akash.volume.v1.ObjectChunk", len)?;
        if !self.key.is_empty() {
            struct_ser.serialize_field("key", &self.key)?;
        }
        if let Some(v) = self.attributes.as_ref() {
            struct_ser.serialize_field("attributes", v)?;
        }
        if self.offset != 0 {
            #[allow(clippy::needless_borrow)]
            #[allow(clippy::needless_borrows_for_generic_args)]
            struct_ser.serialize_field("offset", ToString::to_string(&self.offset).as_str())?;
        }
        if !self.data.is_empty() {
            #[allow(clippy::needless_borrow)]
            #[allow(clippy::needless_borrows_for_generic_args)]
            struct_ser.serialize_field("data", pbjson::private::base64::encode(&self.data).as_str())?;
        }
        if !self.sha256.is_empty() {
            #[allow(clippy::needless_borrow)]
            #[allow(clippy::needless_borrows_for_generic_args)]
            struct_ser.serialize_field("sha256", pbjson::private::base64::encode(&self.sha256).as_str())?;
        }
        if !self.object_sha256.is_empty() {
            #[allow(clippy::needless_borrow)]
            #[allow(clippy::needless_borrows_for_generic_args)]
            struct_ser.serialize_field("objectSha256", pbjson::private::base64::encode(&self.object_sha256).as_str())?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for ObjectChunk {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "key",
            "attributes",
            "offset",
            "data",
            "sha256",
            "object_sha256",
            "objectSha256",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Key,
            Attributes,
            Offset,
            Data,
            Sha256,
            ObjectSha256,
        }
        impl<'de> serde::Deserialize<'de> for GeneratedField {
            fn deserialize<D>(deserializer: D) -> std::result::Result<GeneratedField, D::Error>
            where
                D: serde::Deserializer<'de>,
            {
                struct GeneratedVisitor;

                impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
                    type Value = GeneratedField;

                    fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                        write!(formatter, "expected one of: {:?}", &FIELDS)
                    }

                    #[allow(unused_variables)]
                    fn visit_str<E>(self, value: &str) -> std::result::Result<GeneratedField, E>
                    where
                        E: serde::de::Error,
                    {
                        match value {
                            "key" => Ok(GeneratedField::Key),
                            "attributes" => Ok(GeneratedField::Attributes),
                            "offset" => Ok(GeneratedField::Offset),
                            "data" => Ok(GeneratedField::Data),
                            "sha256" => Ok(GeneratedField::Sha256),
                            "objectSha256" | "object_sha256" => Ok(GeneratedField::ObjectSha256),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = ObjectChunk;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct akash.volume.v1.ObjectChunk")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<ObjectChunk, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut key__ = None;
                let mut attributes__ = None;
                let mut offset__ = None;
                let mut data__ = None;
                let mut sha256__ = None;
                let mut object_sha256__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Key => {
                            if key__.is_some() {
                                return Err(serde::de::Error::duplicate_field("key"));
                            }
                            key__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Attributes => {
                            if attributes__.is_some() {
                                return Err(serde::de::Error::duplicate_field("attributes"));
                            }
                            attributes__ = map_.next_value()?;
                        }
                        GeneratedField::Offset => {
                            if offset__.is_some() {
                                return Err(serde::de::Error::duplicate_field("offset"));
                            }
                            offset__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::Data => {
                            if data__.is_some() {
                                return Err(serde::de::Error::duplicate_field("data"));
                            }
                            data__ = 
                                Some(map_.next_value::<::pbjson::private::BytesDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::Sha256 => {
                            if sha256__.is_some() {
                                return Err(serde::de::Error::duplicate_field("sha256"));
                            }
                            sha256__ = 
                                Some(map_.next_value::<::pbjson::private::BytesDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::ObjectSha256 => {
                            if object_sha256__.is_some() {
                                return Err(serde::de::Error::duplicate_field("objectSha256"));
                            }
                            object_sha256__ = 
                                Some(map_.next_value::<::pbjson::private::BytesDeserialize<_>>()?.0)
                            ;
                        }
                    }
                }
                Ok(ObjectChunk {
                    key: key__.unwrap_or_default(),
                    attributes: attributes__,
                    offset: offset__.unwrap_or_default(),
                    data: data__.unwrap_or_default(),
                    sha256: sha256__.unwrap_or_default(),
                    object_sha256: object_sha256__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("akash.volume.v1.ObjectChunk", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for StatusRequest {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.owner.is_empty() {
            len += 1;
        }
        if !self.vid.is_empty() {
            len += 1;
        }
        if self.dseq != 0 {
            len += 1;
        }
        if !self.requester.is_empty() {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("akash.volume.v1.StatusRequest", len)?;
        if !self.owner.is_empty() {
            struct_ser.serialize_field("owner", &self.owner)?;
        }
        if !self.vid.is_empty() {
            struct_ser.serialize_field("vid", &self.vid)?;
        }
        if self.dseq != 0 {
            #[allow(clippy::needless_borrow)]
            #[allow(clippy::needless_borrows_for_generic_args)]
            struct_ser.serialize_field("dseq", ToString::to_string(&self.dseq).as_str())?;
        }
        if !self.requester.is_empty() {
            struct_ser.serialize_field("requester", &self.requester)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for StatusRequest {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "owner",
            "vid",
            "dseq",
            "requester",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Owner,
            Vid,
            Dseq,
            Requester,
        }
        impl<'de> serde::Deserialize<'de> for GeneratedField {
            fn deserialize<D>(deserializer: D) -> std::result::Result<GeneratedField, D::Error>
            where
                D: serde::Deserializer<'de>,
            {
                struct GeneratedVisitor;

                impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
                    type Value = GeneratedField;

                    fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                        write!(formatter, "expected one of: {:?}", &FIELDS)
                    }

                    #[allow(unused_variables)]
                    fn visit_str<E>(self, value: &str) -> std::result::Result<GeneratedField, E>
                    where
                        E: serde::de::Error,
                    {
                        match value {
                            "owner" => Ok(GeneratedField::Owner),
                            "vid" => Ok(GeneratedField::Vid),
                            "dseq" => Ok(GeneratedField::Dseq),
                            "requester" => Ok(GeneratedField::Requester),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = StatusRequest;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct akash.volume.v1.StatusRequest")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<StatusRequest, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut owner__ = None;
                let mut vid__ = None;
                let mut dseq__ = None;
                let mut requester__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Owner => {
                            if owner__.is_some() {
                                return Err(serde::de::Error::duplicate_field("owner"));
                            }
                            owner__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Vid => {
                            if vid__.is_some() {
                                return Err(serde::de::Error::duplicate_field("vid"));
                            }
                            vid__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Dseq => {
                            if dseq__.is_some() {
                                return Err(serde::de::Error::duplicate_field("dseq"));
                            }
                            dseq__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::Requester => {
                            if requester__.is_some() {
                                return Err(serde::de::Error::duplicate_field("requester"));
                            }
                            requester__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(StatusRequest {
                    owner: owner__.unwrap_or_default(),
                    vid: vid__.unwrap_or_default(),
                    dseq: dseq__.unwrap_or_default(),
                    requester: requester__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("akash.volume.v1.StatusRequest", FIELDS, GeneratedVisitor)
    }
}
impl serde::Serialize for StatusResponse {
    #[allow(deprecated)]
    fn serialize<S>(&self, serializer: S) -> std::result::Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeStruct;
        let mut len = 0;
        if !self.snapshots.is_empty() {
            len += 1;
        }
        if !self.latest_snapshot.is_empty() {
            len += 1;
        }
        if self.size != 0 {
            len += 1;
        }
        if !self.sha256.is_empty() {
            len += 1;
        }
        if self.sync_lag_seconds != 0 {
            len += 1;
        }
        if self.r#final {
            len += 1;
        }
        let mut struct_ser = serializer.serialize_struct("akash.volume.v1.StatusResponse", len)?;
        if !self.snapshots.is_empty() {
            struct_ser.serialize_field("snapshots", &self.snapshots)?;
        }
        if !self.latest_snapshot.is_empty() {
            struct_ser.serialize_field("latestSnapshot", &self.latest_snapshot)?;
        }
        if self.size != 0 {
            #[allow(clippy::needless_borrow)]
            #[allow(clippy::needless_borrows_for_generic_args)]
            struct_ser.serialize_field("size", ToString::to_string(&self.size).as_str())?;
        }
        if !self.sha256.is_empty() {
            #[allow(clippy::needless_borrow)]
            #[allow(clippy::needless_borrows_for_generic_args)]
            struct_ser.serialize_field("sha256", pbjson::private::base64::encode(&self.sha256).as_str())?;
        }
        if self.sync_lag_seconds != 0 {
            #[allow(clippy::needless_borrow)]
            #[allow(clippy::needless_borrows_for_generic_args)]
            struct_ser.serialize_field("syncLagSeconds", ToString::to_string(&self.sync_lag_seconds).as_str())?;
        }
        if self.r#final {
            struct_ser.serialize_field("final", &self.r#final)?;
        }
        struct_ser.end()
    }
}
impl<'de> serde::Deserialize<'de> for StatusResponse {
    #[allow(deprecated)]
    fn deserialize<D>(deserializer: D) -> std::result::Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        const FIELDS: &[&str] = &[
            "snapshots",
            "latest_snapshot",
            "latestSnapshot",
            "size",
            "sha256",
            "sync_lag_seconds",
            "syncLagSeconds",
            "final",
        ];

        #[allow(clippy::enum_variant_names)]
        enum GeneratedField {
            Snapshots,
            LatestSnapshot,
            Size,
            Sha256,
            SyncLagSeconds,
            Final,
        }
        impl<'de> serde::Deserialize<'de> for GeneratedField {
            fn deserialize<D>(deserializer: D) -> std::result::Result<GeneratedField, D::Error>
            where
                D: serde::Deserializer<'de>,
            {
                struct GeneratedVisitor;

                impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
                    type Value = GeneratedField;

                    fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                        write!(formatter, "expected one of: {:?}", &FIELDS)
                    }

                    #[allow(unused_variables)]
                    fn visit_str<E>(self, value: &str) -> std::result::Result<GeneratedField, E>
                    where
                        E: serde::de::Error,
                    {
                        match value {
                            "snapshots" => Ok(GeneratedField::Snapshots),
                            "latestSnapshot" | "latest_snapshot" => Ok(GeneratedField::LatestSnapshot),
                            "size" => Ok(GeneratedField::Size),
                            "sha256" => Ok(GeneratedField::Sha256),
                            "syncLagSeconds" | "sync_lag_seconds" => Ok(GeneratedField::SyncLagSeconds),
                            "final" => Ok(GeneratedField::Final),
                            _ => Err(serde::de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }
                deserializer.deserialize_identifier(GeneratedVisitor)
            }
        }
        struct GeneratedVisitor;
        impl<'de> serde::de::Visitor<'de> for GeneratedVisitor {
            type Value = StatusResponse;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("struct akash.volume.v1.StatusResponse")
            }

            fn visit_map<V>(self, mut map_: V) -> std::result::Result<StatusResponse, V::Error>
                where
                    V: serde::de::MapAccess<'de>,
            {
                let mut snapshots__ = None;
                let mut latest_snapshot__ = None;
                let mut size__ = None;
                let mut sha256__ = None;
                let mut sync_lag_seconds__ = None;
                let mut r#final__ = None;
                while let Some(k) = map_.next_key()? {
                    match k {
                        GeneratedField::Snapshots => {
                            if snapshots__.is_some() {
                                return Err(serde::de::Error::duplicate_field("snapshots"));
                            }
                            snapshots__ = Some(map_.next_value()?);
                        }
                        GeneratedField::LatestSnapshot => {
                            if latest_snapshot__.is_some() {
                                return Err(serde::de::Error::duplicate_field("latestSnapshot"));
                            }
                            latest_snapshot__ = Some(map_.next_value()?);
                        }
                        GeneratedField::Size => {
                            if size__.is_some() {
                                return Err(serde::de::Error::duplicate_field("size"));
                            }
                            size__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::Sha256 => {
                            if sha256__.is_some() {
                                return Err(serde::de::Error::duplicate_field("sha256"));
                            }
                            sha256__ = 
                                Some(map_.next_value::<::pbjson::private::BytesDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::SyncLagSeconds => {
                            if sync_lag_seconds__.is_some() {
                                return Err(serde::de::Error::duplicate_field("syncLagSeconds"));
                            }
                            sync_lag_seconds__ = 
                                Some(map_.next_value::<::pbjson::private::NumberDeserialize<_>>()?.0)
                            ;
                        }
                        GeneratedField::Final => {
                            if r#final__.is_some() {
                                return Err(serde::de::Error::duplicate_field("final"));
                            }
                            r#final__ = Some(map_.next_value()?);
                        }
                    }
                }
                Ok(StatusResponse {
                    snapshots: snapshots__.unwrap_or_default(),
                    latest_snapshot: latest_snapshot__.unwrap_or_default(),
                    size: size__.unwrap_or_default(),
                    sha256: sha256__.unwrap_or_default(),
                    sync_lag_seconds: sync_lag_seconds__.unwrap_or_default(),
                    r#final: r#final__.unwrap_or_default(),
                })
            }
        }
        deserializer.deserialize_struct("akash.volume.v1.StatusResponse", FIELDS, GeneratedVisitor)
    }
}
