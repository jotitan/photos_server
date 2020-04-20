import React, {useEffect, useState} from 'react'
import {Col, Popconfirm, Row, Tooltip} from 'antd'
import Gallery from 'react-grid-gallery'
import axios from "axios";
import {getBaseUrl,getBaseUrlHref} from "../treeFolder";
import {DeleteFilled, ReloadOutlined,FileImageOutlined, PictureOutlined} from "@ant-design/icons";

export default function MyGallery({urlFolder,refresh}) {
    const [images,setImages] = useState([]);
    const [updateUrl,setUpdateUrl] = useState('');
    const [currentImage,setCurrentImage] = useState(-1);
    const [updateRunning,setUpdateRunning] = useState(false);
    const [key,setKey] = useState(-1);
    const [lightboxVisible,setLightboxVisible] = useState(false);
    const [canDelete,setCanDelete] = useState(false);
    const [showThumbnails,setShowThumbnails] = useState(false);
    const [comp,setComp] = useState(null);
    let baseUrl = getBaseUrl();
    let baseUrlHref = getBaseUrlHref();

    useEffect(()=>{
        if(comp!=null){
            setTimeout(()=>comp.onResize(),300);
        }
    },[refresh,comp])

    useEffect(()=>{
        axios({
            method:'GET',
            url:baseUrl+'/canDelete',
        }).then(d=>{
            setCanDelete(d.data.can);
            if(d.data.can){
                window.addEventListener('keydown',e=>{
                    if(e.key === "t"){
                        // Switch thumbnail
                        setShowThumbnails(s=> !s);
                    }
                    setKey(e.key)
                });
            }
        });
    },[baseUrl,setShowThumbnails]);

    useEffect(()=>{
        if(lightboxVisible && key === "Delete"){
            images[currentImage].isSelected=true;
            setKey("");
        }
    },[currentImage,key,lightboxVisible,images]);

    const loadImages = url => {
        if(url === ''){return;}
        axios({
            method:'GET',
            url:url,
        }).then(d=>{
            // Filter image by time before
            setUpdateUrl(d.data.UpdateUrl)
            setImages(d.data.Files
                .filter(file=>file.ImageLink != null)
                .sort((img1,img2)=>new Date(img1.Date) - new Date(img2.Date))
                .map(img=>{
                    let d = new Date(img.Date).toLocaleString();
                    return {
                        hdLink:baseUrlHref + img.HdLink,
                        path:img.HdLink,
                        Date:d,
                        caption:"",thumbnail:baseUrl + img.ThumbnailLink,src:baseUrl + img.ImageLink,
                        customOverlay:<div style={{padding:2+'px',bottom:0,opacity:0.8,fontSize:10+'px',position:'absolute',backgroundColor:'white'}}>{d}</div>,
                        thumbnailWidth:img.Width,
                        thumbnailHeight:img.Height
                    }
                }));
        })
    };

    useEffect(()=>loadImages(urlFolder),[urlFolder]);

    const selectImage = index=>{
        setImages(list=>{
            let copy = list.slice();
            copy[index].isSelected = list[index].isSelected != null ? !list[index].isSelected : true;
            return copy;
        });
    };

    const deleteSelection = ()=>{
        axios({
            method:'POST',
            url:baseUrl + '/delete',
            data:JSON.stringify(images.filter(i=>i.isSelected).map(i=>i.path))
        }).then(r=>{
            if(r.data.errors === 0) {
                setImages(images.filter(i => !i.isSelected));
            }
        });
    };

    const updateFolder = ()=> {
        if(canDelete && updateUrl !==""){
            setUpdateRunning(true);
            axios({
                method:'GET',
                url:baseUrl + updateUrl,
            }).then(()=>{
                // Reload folder
                loadImages(urlFolder);
                setUpdateRunning(false);
            })
        }
    };

    // Show informations about selected images
    const showSelected = ()=>{
        const selected = images.filter(i=>i.isSelected).length;
        return selected > 0 ? <>
            <Popconfirm placement="bottom" title={"Es tu sûr de vouloir supprimer ces photos"}
                        onConfirm={deleteSelection} okText="Oui" cancelText="Non">
                <Tooltip key={"image-info"} placement="top" title={"Supprimer la sélection"} overlayStyle={{zIndex:20000}}>
                    <DeleteFilled style={{cursor:'pointer'}}/>
                </Tooltip>
                <span style={{marginLeft:10+'px'}}>{selected}</span>
            </Popconfirm>
        </>:''
    };

    const showUpdateLink = ()=> {
        return !canDelete || updateUrl === '' ? <></> :
            <>
                <Popconfirm placement="bottom" title={"Es tu sûr de vouloir mettre à jour le répertoire"}
                            onConfirm={updateFolder} okText="Oui" cancelText="Non">
                    <Tooltip key={"image-info"} placement="top" title={"Mettre à jour le répertoire"}>
                        <ReloadOutlined spin={updateRunning} />
                    </Tooltip>
                </Popconfirm>
            </>;
    }

    // Add behaviour when show image in lightbox
    const getCustomActions = ()=> {
        return [
            <div style={{paddingTop:5+'px'}} key={"detail-lightbox"}>
                <Tooltip key={"image-info"} placement="top" title={"Télécharger en HD"} overlayStyle={{zIndex:20000}}>
                    <a target={"_blank"} rel="noopener noreferrer"
                       download={images != null && currentImage !== -1 ? images[currentImage].Name:''}
                       href={images != null && currentImage !== -1 ? images[currentImage].hdLink:''} >
                        <FileImageOutlined style={{color:'white',fontSize:22+'px'}}/>
                    </a>
                </Tooltip>
                <span style={{color:'white',paddingLeft:20+'px'}}>
                   {images!=null && currentImage!==-1 ? images[currentImage].Date:''}
               </span>
            </div>
        ]
    };
    return (
        <>
            <Row className={"options"}>
                <Col span={8}>
                    {images.length} <PictureOutlined />
                </Col>
                <Col span={8}>
                    {showSelected()}
                </Col>

                <Col span={8}>
                    {showUpdateLink()}
                </Col>
            </Row>
            <Row className={"gallery"}>
                <Col span={24} style={{marginTop:30+'px'}}>
                    <Gallery ref={node=>{setComp(node);window.t = node}}
                        images={images}
                        imageCountSeparator={" / "}
                        showImageCount={false}
                        lightboxWillClose={()=>setLightboxVisible(false)}
                        lightboxWillOpen={()=>setLightboxVisible(true)}
                        onSelectImage={selectImage}
                        enableImageSelection={canDelete===true}
                        currentImageWillChange={indexImage=>setCurrentImage(indexImage)}
                        customControls={getCustomActions()}
                        showLightboxThumbnails={showThumbnails}
                        backdropClosesModal={true} lightboxWidth={2000}/>
                </Col>
            </Row>
        </>
    )
}