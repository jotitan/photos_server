import React, {useEffect, useState} from 'react'
import {Input, Popover, Tag, Tree} from 'antd'
import axios from "axios";
import useLocalStorage from "../../services/local-storage.hook";
import {FilterOutlined} from "@ant-design/icons";
import './treeFolder.css'

export const getBaseUrlHref = ()=>getBaseUrl(window.location.href);

export const getBaseUrl = (defaultValue=window.location.origin)=>{
    const hr = window.location.href;
    switch (window.location.hostname) {
        case 'localhost':
            // To manage also proxy locally and remove last / if necessary
            if(hr.endsWith("/")){
                return hr.substring(0, hr.length -1)
            }
            return hr;
        default : return defaultValue;
    }
}

const { Search } = Input;

const sortByName = (a,b)=>a.Name === b.name ? 0:a.Name < b.Name ? -1:1;

const adapt = node => {
    let data = {
        title:node.Name.replace(/_/g," "),
        id:node.Id,
        key:getBaseUrl() + node.Link,
        path:node.Path,
        tags:getBaseUrl() + node.LinkTags,
        hasImages:node.HasImages
    };

    if(node.Children != null && node.Children.length > 0){
        data.children = node.Children.sort(sortByName).map(adapt);
    }else{
        data.isLeaf=true
    }
    return data;
};

export default function TreeFolder({setUrlFolder,setTitleGallery,update,canFilter,rootUrl,filterMode}) {
    const [tree,setTree] = useState([]);
    const [originalTree,setOriginalTree] = useState([]);
    const { DirectoryTree } = Tree;
    const [height,setHeight] = useState(window.innerHeight-185);
    const [peoples,setPeoples] = useState([]);
    const [selectedPeopleFilter,setSelectedPeopleFilter] = useState(null);
    const [expandables,setExpandables] = useLocalStorage("expandables",[])
    useEffect(()=>{
        loadPeoplesTag();
        axios({
            method:'GET',
            url:`${getBaseUrl()}${rootUrl}`,
        }).then(d=>{
            if(d != null) {
                let data = d.data.map(adapt);
                setTree(data);
                setOriginalTree(data);
            }
        })
    },[update,rootUrl]);

    const hasName = (value,node,root,paths)=>{
        if(node.title.toLowerCase().indexOf(value) !== -1 || paths[root] != null){
            return node;
        }
        if(node.children == null || node.children.length === 0){
            // Leaf
            return null;
        }else{
            let children = node.children.map(a=>hasName(value,a,root + '/' + a.title,paths)).filter(a=>a!=null);
            if(children.length > 0){
                return {title:node.title,children:children,key:node.key,tags:node.tags,hasImages:node.hasImages};
            }
            return null;
        }
    };

    const hasId = (set,node)=>{
        if(set.has(node.id)){
            return node;
        }
        if(node.children == null || node.children.length === 0){
            // Leaf
            return null;
        }else{
            let children = node.children.map(a=>hasId(set,a)).filter(a=>a!=null);
            if(children.length > 0){
                return {title:node.title,children:children,key:node.key,tags:node.tags,hasImages:node.hasImages};
            }
            return null;
        }
    };

    const filter = value => {
        if(filterMode === 'video') {
            setUrlFolder({load: `/video/search?query=${value}`, path: ''});
        }else {
            // ask server
            axios({
                url: getBaseUrl() + `/filterTagsFolder?value=${value}`
            }).then(d => {
                // Create map
                let map = {};
                d.data.forEach(d => map[d.replace(/_/g, " ")] = true);
                // Keep values from server and from name
                let values = originalTree.map(n => hasName(value.toLowerCase(), n, n.title, map)).filter(n => n != null);
                setTree(values);
            })
        }
    };

    const filterTree = event => {
        if(event.key === "Enter"){
            filter(event.target.value);
        }
    };

    const onSelect = (e,f)=>{
        setTitleGallery(f.node.title);
        if(f.node.children == null || f.node.children.length === 0) {
            setUrlFolder({load:e[0],tags:f.node.tags,path:f.node.path})
        }else{
            // Case when folder has sub folders but also images
            if(f.node.hasImages){
                setUrlFolder({load:e[0],tags:f.node.tags,path:f.node.path})
            }
        }
    };

    const filterFolder = idTag=>{
        if(idTag === selectedPeopleFilter){
            setSelectedPeopleFilter(null)
            // Unselect and show all folders
            return setTree(originalTree)
        }
        setSelectedPeopleFilter(idTag)
        axios({
            url:`${getBaseUrl()}/tag/filter_folder?tag=${idTag}`,
            method:'GET'
        }).then(data=>{
            // Hide folder not returned
            const s = new Set(data.data)
            let values = originalTree.map(n => hasId(s,n)).filter(n => n != null)
            setTree(values)
        })
    }

    window.addEventListener('resize', ()=>setHeight(window.innerHeight-185));

    const loadPeoplesTag = ()=>{
        axios({
            method: 'GET',
            url: getBaseUrl() + '/tag/peoples',
        }).then(data=>setPeoples(data.data))
    }
    const showFilterFolder = ()=>{
        return <Popover trigger={'click'} title={'Filter folders'} content={
            peoples.map(p=>
                <Tag className={selectedPeopleFilter === p.id?"filter-selected":""} style={{cursor:'pointer'}}
                     onClick={()=>filterFolder(p.id)}>
                    {p.name}
                </Tag>)
        }>
            <FilterOutlined style={{backgroundColor:'white',color:'#001529',padding:5}}/>
        </Popover>
    }

    return(
        <>
            {canFilter ?
                <>
                    <Search onKeyUp={filterTree} size={"small"}
                            placeholder={"Filtrer par tag ou par nom"}
                            style={{marginLeft:10,width:255,marginRight:10}}/>
                    {showFilterFolder()}
                </>
                :<></>
            }
            {tree.length > 0 ? <DirectoryTree
                onSelect={onSelect}
                treeData={tree}
                height={height}
                autoExpandParent={false}
                expandedKeys={expandables}
                onExpand={setExpandables}
                virtual={true}
                style={{
                    fontSize: 12 + 'px',
                    width: 300 + 'px',
                    overflow: 'auto',
                    backgroundColor: '#001529',
                    color: '#999'
                }}
            />:''}
        </>
    )
}
