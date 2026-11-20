import React, {useEffect, useState} from 'react'
import {Col, Empty, Modal, Row, Tooltip} from 'antd'
import Gallery from 'react-grid-gallery'
import {getBaseUrl, getBaseUrlHref} from "../treeFolder";
import './gallery.css'

import {CloseOutlined, FileImageOutlined, PlusCircleOutlined,} from "@ant-design/icons";
import {TransformComponent, TransformWrapper} from "react-zoom-pan-pinch";

const setProperty = (ctx, property, value) => {
    const copy = {...ctx};
    copy[property] = value;
    return copy;
}

const adaptImages = photos => {
    return photos
        .filter(file => file.ImageLink != null)
        .sort((img1, img2) => new Date(img1.Date) - new Date(img2.Date))
        .map(img => {
            let d = new Date(img.Date).toLocaleString();
            let folder = img.HdLink.replace(img.Name, '').replace('/imagehd/', '');
            return {
                hdLink: getBaseUrlHref() + img.HdLink,
                path: img.HdLink,
                folder: folder,
                Date: d,
                caption: "", thumbnail: getBaseUrl() + img.ThumbnailLink, src: getBaseUrl() + img.ImageLink,
                customOverlay: <div style={{
                    padding: 2 + 'px',
                    bottom: 0,
                    opacity: 0.8,
                    fontSize: 10 + 'px',
                    position: 'absolute',
                    backgroundColor: 'white'
                }}>{d}</div>,
                thumbnailWidth: img.Width || 0,
                thumbnailHeight: img.Height || 0
            }
        });
};

// setIsAddFolderPanelVisible to show folder to upload
export default function SearchGallery({
                                      hideSearchGallery
                                  }) {
    const [images, setImages] = useState([]);
    const [imageToZoom, setImageToZoom] = useState('');
    const [zoomEnable, setZoomEnable] = useState(false);
    const [currentImage, setCurrentImage] = useState(-1);

    const loadImages = photos => setImages(adaptImages(photos));

    // Add behaviour when show image in lightbox
    const getCustomActions = () => {
        return [
            <div style={{paddingTop: 5 + 'px'}} key={"detail-lightbox"}>
                <Tooltip key={"download-image-info"} placement="top" title={"Télécharger en HD"}
                         overlayStyle={{zIndex: 20000}}>
                    <a target={"_blank"} rel="noopener noreferrer"
                       download={images != null && currentImage !== -1 ? images[currentImage].Name : ''}
                       href={images != null && currentImage !== -1 ? images[currentImage].hdLink : ''}>
                        <FileImageOutlined style={{color: 'white', fontSize: 22 + 'px'}}/>
                    </a>
                </Tooltip>
                <Tooltip key={"zoom-image-info"} placement="top" title={"Zoom"} overlayStyle={{zIndex: 20000}}>
                    <PlusCircleOutlined style={{color: 'white', fontSize: 22 + 'px', marginLeft: 5}}
                                        onClick={() => setZoomEnable(true)}/>
                </Tooltip>
                <span style={{color: 'white', paddingLeft: 20 + 'px'}}>
                   {images != null && currentImage !== -1 ? images[currentImage].Date : ''}
                    {images != null && currentImage !== -1 ? ' - ' + images[currentImage].folder : ''}
               </span>
            </div>
        ]
    };

    const showGallery = () => {
        return <>
            <Gallery
                     images={images}
                     showImageCount={false}
                     currentImageWillChange={indexImage => {
                         setCurrentImage(indexImage)
                         setImageToZoom(images[indexImage].hdLink);
                     }}
                     customControls={getCustomActions()}
                     backdropClosesModal={true} lightboxWidth={2000}/>
        </>
    };

    const showEmptyMessage = () => {
        return (
            images.length >  0 ? <></> :
                <Empty style={{marginTop: '40vh'}}
                       description={<span style={{color: 'white', fontWeight: 'bold'}}>Pas de photos</span>}
                       image={Empty.PRESENTED_IMAGE_SIMPLE}/>
        );
    };
    const doSearch = value => {
        fetch(`${getBaseUrl()}/photo/search?q=${value}`)
            .then(d=>d.json())
            .then(loadImages);
    }
    
    const search = event => {
        if(event.key === 'Enter') {
            doSearch(event.target.value)
        }
    }

    const [scale, setScale] = useState(1);
    const [posX, setPosX] = useState(1);
    const [posY, setPosY] = useState(1);
    return (
        <>
            <Row style={{backgroundColor: 'var(--main-bg-color)'}}>
                <Col flex={'100%'}
                     style={{height:50,borderBottom:'solid 4px grey'}}>
                    <input placeholder={"Recherche..."} size={30} style={{marginTop:10,marginLeft:20,width:500}} onKeyUp={search}/>
                    <span style={{marginLeft:20,color:'white'}}>Résultats : <span style={{fontWeight:'bold'}}>{images.length}</span></span>
                    <button style={{position:'absolute',right:10,top:10}} onClick={()=>hideSearchGallery()}>Fermer</button>
                </Col>
            </Row>
            <Row className={"gallery"}>
                <Col flex={'100%'}
                     style={{marginTop: 10}}>
                    {images.length === 0 ? showEmptyMessage() : showGallery()}
                </Col>
                <Modal visible={zoomEnable}
                       onCancel={() => {
                           setScale(1);
                           setPosX(0);
                           setPosY(0);
                           setZoomEnable(false)
                       }}
                       width={90 + '%'}
                       style={{top: 20}}
                       footer={[]}
                       closeIcon={<CloseOutlined style={{color: 'white', fontSize: 20}}/>}
                       wrapClassName={"modal-zoom"}>
                    <TransformWrapper scale={scale} positionX={posX} positionY={posY}>
                        <TransformComponent>
                            <img src={imageToZoom} alt="Zoomed panel"/>
                        </TransformComponent>
                    </TransformWrapper>
                </Modal>
            </Row>
        </>
    )
}
