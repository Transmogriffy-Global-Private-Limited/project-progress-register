ALTER TABLE public.progress_attachments
    DROP CONSTRAINT attachment_camera_is_image,
    DROP CONSTRAINT attachment_verified_shape;

ALTER TABLE public.progress_attachments
    ADD CONSTRAINT attachment_camera_is_supported_media
    CHECK (source <> 'camera' OR media_kind IN ('image', 'video')),
    ADD CONSTRAINT attachment_verified_shape
    CHECK (verification_status <> 'verified' OR (source = 'camera' AND media_kind IN ('image', 'video')));
