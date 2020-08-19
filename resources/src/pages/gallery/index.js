import React, {useEffect, useState, useCallback} from 'react'
import {Col, Input, Modal, notification, Popconfirm, Row, Tag, Tooltip} from 'antd'
import Gallery from 'react-grid-gallery'
import axios from "axios";
import {getBaseUrl,getBaseUrlHref} from "../treeFolder";

import {
    DeleteFilled,
    ReloadOutlined,
    FileImageOutlined,
    PictureOutlined,
    DeleteTwoTone,
    PlusOutlined,
    PlusCircleOutlined,
    CloseOutlined,
    ChromeOutlined
} from "@ant-design/icons";
import {TransformWrapper,TransformComponent} from "react-zoom-pan-pinch";


import { CirclePicker } from 'react-color';

// setIsAddFolderPanelVisible to show folder to upload
export default function MyGallery({urlFolder,refresh,titleGallery,canAdmin,setIsAddFolderPanelVisible,setCurrentFolder,update}) {
    const [images,setImages] = useState([]);
    const [imageToZoom,setImageToZoom] = useState('');
    const [zoomEnable,setZoomEnable] = useState(false);
    const [updateUrl,setUpdateUrl] = useState('');
    const [updateExifUrl,setUpdateExifUrl] = useState('');
    const [currentImage,setCurrentImage] = useState(-1);
    const [updateRunning,setUpdateRunning] = useState(false);
    const [updateExifRunning,setUpdateExifRunning] = useState(false);
    const [key,setKey] = useState(-1);
    const [lightboxVisible,setLightboxVisible] = useState(false);
    const [showThumbnails,setShowThumbnails] = useState(false);
    const [comp,setComp] = useState(null);
    let baseUrl = getBaseUrl();
    let baseUrlHref = getBaseUrlHref();

    // Gestion du tag
    const [showInputTag,setShowInputTag] = useState(false);
    const [tags,setTags] = useState([]);
    const [nextTagValue,setNextTagValue] = useState('');

    useEffect(()=>{
        if(comp!=null){
            setTimeout(()=>comp.onResize(),300);
        }
    },[refresh,comp])

    useEffect(()=>{
        if(canAdmin){
            window.addEventListener('keydown',e=>{
                if(e.key === "t"){
                    // Switch thumbnail
                    setShowThumbnails(s=> !s);
                }
                setKey(e.key)
            });
        }
    },[canAdmin,setShowThumbnails]);

    useEffect(()=>{
        if(lightboxVisible && key === "Delete"){
            images[currentImage].isSelected=!images[currentImage].isSelected;
            setKey("");
        }
    },[currentImage,key,lightboxVisible,images]);

    const saveTag = (tag,callback) => {
        axios({
            method:'POST',
            url:urlFolder.tags,
            data:JSON.stringify(tag),
        }).then(callback);
    }

    const memLoadImages = useCallback(()=> {
        if(urlFolder === '' || urlFolder.load === ''){return;}
        axios({
            method:'GET',
            url:urlFolder.load,
        }).then(d=>{
            // Filter image by time before
            setUpdateUrl(d.data.UpdateUrl);
            setUpdateExifUrl(d.data.UpdateExifUrl);
            setCurrentFolder(d.data.FolderPath);
            let photos = d.data.Files != null ? d.data.Files:d.data;
            setTags(d.data.Tags.map(t=>{return {value:t.Value,color:t.Color}}));
            setImages(photos
                .filter(file=>file.ImageLink != null)
                .sort((img1,img2)=>new Date(img1.Date) - new Date(img2.Date))
                .map(img=>{
                    let d = new Date(img.Date).toLocaleString();
                    let folder = img.HdLink.replace(img.Name,'').replace('/imagehd/','');
                    return {
                        hdLink:baseUrlHref + img.HdLink,
                        path:img.HdLink,
                        folder:folder,
                        Date:d,
                        caption:"",thumbnail:baseUrl + img.ThumbnailLink,src:baseUrl + img.ImageLink,
                        customOverlay:<div style={{padding:2+'px',bottom:0,opacity:0.8,fontSize:10+'px',position:'absolute',backgroundColor:'white'}}>{d}</div>,
                        thumbnailWidth:img.Width,
                        thumbnailHeight:img.Height
                    }
                }));
        })
    },[urlFolder,baseUrl,baseUrlHref,setCurrentFolder]);

    useEffect(()=>memLoadImages(urlFolder),[urlFolder,memLoadImages,update]);

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
                let count = images.filter(i => i.isSelected).length;
                setImages(images.filter(i => !i.isSelected));
                notification["success"]({message:"Succès",description:`${count} images ont été bien supprimées`});
            }
        });
    };

    const updateFolder = ()=> {
        if(canAdmin && updateUrl !==""){
            setUpdateRunning(true);
            axios({
                method:'GET',
                url:baseUrl + updateUrl,
            }).then(()=>{
                // Reload folder
                memLoadImages(urlFolder);
                setUpdateRunning(false);
            })
        }
    };

    const updateExifFolder = ()=> {
        if(canAdmin && updateExifUrl !==""){
            setUpdateExifRunning(true);
            axios({
                method:'GET',
                url:baseUrl + updateExifUrl,
            }).then(()=>{
                // Reload folder
                memLoadImages(urlFolder);
                setUpdateExifRunning(false);
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
                    <DeleteFilled className={"button"}/>
                </Tooltip>
                <span style={{marginLeft:10+'px'}}>{selected}</span>
            </Popconfirm>
        </>:''
    };

    const addPhotosToFolder = ()=> {
        setIsAddFolderPanelVisible(true);
    };

    const showUpdateLink = ()=> {
        return !canAdmin || updateUrl === '' || updateUrl ==null ? <></> :
            <>
                <Popconfirm placement="bottom" title={"Es tu sûr de vouloir mettre à jour les Exifs"}
                            onConfirm={updateExifFolder} okText="Oui" cancelText="Non">
                    <Tooltip key={"image-info"} placement="top" title={"Mettre à jour les Exifs"}>
                        <ChromeOutlined spin={updateExifRunning} className={"button"}/>
                    </Tooltip>
                </Popconfirm>
                <Popconfirm placement="bottom" title={"Es tu sûr de vouloir mettre à jour le répertoire"}
                            onConfirm={updateFolder} okText="Oui" cancelText="Non">
                    <Tooltip key={"image-info"} placement="top" title={"Mettre à jour le répertoire"}>
                        <ReloadOutlined style={{marginLeft:10}} spin={updateRunning} className={"button"}/>
                    </Tooltip>
                </Popconfirm>
                <Tooltip key={"image-info"} placement="top" title={"Ajouter des photos"}>
                    <PlusCircleOutlined className={"button"} style={{marginLeft:10}} onClick={addPhotosToFolder}/>
                </Tooltip>
            </>;
    };

    const updateText = value=>{
        switch(value.key){
            case 'Enter':
                let tag = {value:nextTagValue,color:'green'};
                setShowInputTag(false);
                setNextTagValue('');
                saveTag(tag,()=>setTags(list=>[...list,tag]));
                break;
            default:
                setNextTagValue(value.target.value);
        }
    }

    const updateColor = (color,tag)=>{
        let newTag = {value:tag.value,color:color.hex};
        saveTag(newTag,()=>setTags(tags=>[...tags.filter(n=>n.value !== tag.value),newTag]))
    };

    const removeTag = (tag)=>{
        saveTag({Value:tag.value,Color:tag.color,ToRemove:true});
    };

    // Add behaviour when show image in lightbox
    const getCustomActions = ()=> {
        return [
            <div style={{paddingTop:5+'px'}} key={"detail-lightbox"}>
                {images!=null && currentImage !== -1 && images[currentImage].isSelected? <DeleteTwoTone twoToneColor={"red"} style={{color:'red',fontSize:22+'px'}} />:''}
                <Tooltip key={"image-info"} placement="top" title={"Télécharger en HD"} overlayStyle={{zIndex:20000}}>
                    <a target={"_blank"} rel="noopener noreferrer"
                       download={images != null && currentImage !== -1 ? images[currentImage].Name:''}
                       href={images != null && currentImage !== -1 ? images[currentImage].hdLink:''} >
                        <FileImageOutlined style={{color:'white',fontSize:22+'px'}}/>
                    </a>
                </Tooltip>
                <Tooltip key={"image-info"} placement="top" title={"Zoom"} overlayStyle={{zIndex:20000}}>
                    <PlusCircleOutlined style={{color:'white',fontSize:22+'px',marginLeft:5}} onClick={()=>setZoomEnable(true)}/>
                </Tooltip>
                <span style={{color:'white',paddingLeft:20+'px'}}>
                   {images!=null && currentImage!==-1 ? images[currentImage].Date:''}
                    {images!=null && currentImage!==-1 ? ' - ' + images[currentImage].folder:''}
               </span>
            </div>
        ]
    };

    const [scale,setScale] = useState(1);
    const [posX,setPosX] = useState(1);
    const [posY,setPosY] = useState(1);
    return (
        <>
            <Row className={"options"}>
                <Col flex={"200px"}>
                    {titleGallery}
                    {images.length} <PictureOutlined />
                </Col>
                <Col flex={"100px"}>
                    {showSelected()}
                </Col>
                <Col flex={"100px"}>
                    {showUpdateLink()}
                </Col>
                <Col flex={"auto"}>
                    {
                        tags
                            .sort((a,b)=>a.value < b.value ? -1:1)
                            .map(t=>
                                <Tooltip key={`tp${t.value}`} trigger={"click"} title={
                                    <CirclePicker width={'250px'} onChange={color=>updateColor(color,t)} circleSize={26} circleSpacing={8}/>
                                }><Tag key={t.value} color={t.color} closable={true} onClose={()=>removeTag(t)}>{t.value}</Tag>
                                </Tooltip>
                            )}
                    {!showInputTag && urlFolder.load !== ''?<Tag color="gray" onClick={()=>setShowInputTag(true)}><PlusOutlined /> tag</Tag>:<></>}
                    {showInputTag ? <Input size={"small"} style={{width:78+'px'}} onKeyUp={updateText} autoFocus={true} />:<></>}
                </Col>
            </Row>
            <Row className={"gallery"}>
                <Col span={24} style={{marginTop:36+'px'}}>
                    <Gallery ref={node=>{setComp(node);window.t = node}}
                             images={images}
                        //imageCountSeparator={" / "}
                             showImageCount={false}
                             lightboxWillClose={()=>setLightboxVisible(false)}
                             lightboxWillOpen={()=>setLightboxVisible(true)}
                             onSelectImage={selectImage}
                             enableImageSelection={canAdmin===true}
                             currentImageWillChange={indexImage=>{
                                 setCurrentImage(indexImage)
                                 setImageToZoom(images[indexImage].hdLink);
                             }}
                             customControls={getCustomActions()}
                             showLightboxThumbnails={showThumbnails}
                             backdropClosesModal={true} lightboxWidth={2000}/>
                </Col>
                <Modal visible={zoomEnable}
                       onCancel={()=>{
                           setScale(1);
                           setPosX(0);
                           setPosY(0);
                           setZoomEnable(false)}}
                       width={90+'%'}
                       style={{top:20}}
                       footer={[]}
                       closeIcon={<CloseOutlined style={{color:'white',fontSize:20}}/>}
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